/*
Copyright 2018 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	informers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1beta1"
	listers "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
)

// crdSchemaController collect schemas for all CRDs and update schema map.
type crdSchemaController struct {
	crdClient client.CustomResourceDefinitionsGetter
	crdLister listers.CustomResourceDefinitionLister
	crdSynced cache.InformerSynced

	// To allow injection for testing.
	syncFn func(key string) error

	queue workqueue.RateLimitingInterface

	schemas map[string]v1beta1.JSONSchemaProps
}

// NewCrdSchemaController creates new crdSchemaController.
func NewCrdSchemaController(crdInformer informers.CustomResourceDefinitionInformer,
	crdClient client.CustomResourceDefinitionsGetter) *crdSchemaController {
	ec := &crdSchemaController{
		crdClient: crdClient,
		crdLister: crdInformer.Lister(),
		crdSynced: crdInformer.Informer().HasSynced,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CRDSchemaController"),
	}

	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crd, ok := obj.(*v1beta1.CustomResourceDefinition)
			if ok {
				ec.QueueCRD(crd.Name, time.Second)
			}
		},
		DeleteFunc: func(obj interface{}) {
		},
		UpdateFunc: func(old, new interface{}) {
			crd, ok := new.(*v1beta1.CustomResourceDefinition)
			if ok {
				ec.QueueCRD(crd.Name, time.Second)
			}
		},
	})

	ec.syncFn = ec.sync
	ec.schemas = map[string]v1beta1.JSONSchemaProps{}

	return ec
}

func (ec *crdSchemaController) QueueCRD(key string, timeout time.Duration) {
	ec.queue.AddAfter(key, timeout)
}

func (ec *crdSchemaController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ec.queue.ShutDown()

	glog.Infof("Starting CRDSchemaController")
	defer glog.Infof("Shutting down CRDSchemaController")

	if !cache.WaitForCacheSync(stopCh, ec.crdSynced) {
		return
	}

	// only start one worker thread since its a slow moving API
	go wait.Until(ec.runWorker, time.Second, stopCh)

	<-stopCh
}

func (ec *crdSchemaController) runWorker() {
	for ec.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.
// It returns false when it's time to quit.
func (ec *crdSchemaController) processNextWorkItem() bool {
	key, quit := ec.queue.Get()
	if quit {
		return false
	}
	defer ec.queue.Done(key)

	err := ec.syncFn(key.(string))
	if err == nil {
		ec.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with: %v", key, err))
	ec.queue.AddRateLimited(key)

	return true
}

func (ec *crdSchemaController) sync(key string) error {
	glog.V(4).Infof("checking schema for %v", key)
	cachedCRD, err := ec.crdLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, v := range cachedCRD.Spec.Versions {
		if v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
			key := cachedCRD.Spec.Group + "/" + v.Name + "/" + cachedCRD.Spec.Names.Kind
			glog.V(4).Infof("Adding schema for %v", key)
			ec.schemas[key] = *v.Schema.OpenAPIV3Schema
		}
	}

	return nil
}

func (ec *crdSchemaController) GetSchema(apiVerison, kind string) *v1beta1.JSONSchemaProps {
	ret := ec.schemas[apiVerison+"/"+kind]
	return &ret
}
