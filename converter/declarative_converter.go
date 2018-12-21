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

package converter

import (
	"fmt"
	"github.com/mbohlool/kcrdc/types"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"net/http"
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DeclarativeConverter struct {
	schemaAccess types.CRDSchemaAccess
}

func NewDeclarativeConverterHandler(schemaAccess types.CRDSchemaAccess) http.Handler {
	return &DeclarativeConverter{schemaAccess}
}

func (dc *DeclarativeConverter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	Serve(w, r, dc.convert)
}

type conversion struct {
	method string
	fromVersion string
	params []string
}

// extractConversionTags parses the description to find conversion tags.
// conversion tags are in this format:
// +conversion:$from_version,$method_name,$params...
func extractConversionTags(description string) []conversion {
	marker := "+conversion:"
	var out []conversion
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		line = strings.Trim(line, " ")
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, marker) {
			continue
		}
		var params []string
		kv := strings.SplitN(line[len(marker):], ",", 3)
		if len(kv) <= 1 {
			continue
		}
		if len(kv) == 3 {
			params = strings.Split(kv[2], ",")
		}
		out = append(out, conversion{kv[1], kv[0], params})
	}
	return out
}

func convertRecursive(Object map[string]interface{}, toSubSchema *v1beta1.JSONSchemaProps, fromVersion string) (map[string]interface{}, error) {
	outObject := map[string]interface{}{}
	for k, v := range Object {
		toPropSchema, exists := toSubSchema.Properties[k]
		vMap, ok := v.(map[string]interface{})
		if ok && exists {
			outPropObject, err := convertRecursive(vMap, &toPropSchema, fromVersion)
			if err != nil {
				return nil, err
			}
			outObject[k] = outPropObject
		} else {
			outObject[k] = v
		}
	}
	for k, toPropSchema := range toSubSchema.Properties {
		conversions := extractConversionTags(toPropSchema.Description)
		for _, c := range conversions {
			if fromVersion == c.fromVersion {
				switch c.method {
				// TODO: Rename does not need to be specified on both schemas
				case "rename":
					if len(c.params) != 1 {
						return nil, fmt.Errorf("rename conversion only accept one parameter, actual: %v", c.params)
					}
					if _, exists := Object[c.params[0]]; !exists {
						continue
					}
					outObject[k] = Object[c.params[0]]
					delete(outObject, c.params[0])
				default:
					return nil, fmt.Errorf("conversion method is not supported: %v", c.method)
				}
			}
		}
	}
	return outObject, nil
}

func (dc *DeclarativeConverter) convert(Object *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, metav1.Status) {
	fromVersion := Object.GetAPIVersion()
	kind := Object.GetKind()
	glog.Infof("converting %v from %v to %v.", kind, fromVersion, toVersion)
	toSchema := dc.schemaAccess.GetSchema(toVersion, kind)
	if toSchema == nil {
		return nil, statusErrorWithMessage("declarative conversion to version %v is not possible. missing schema", toVersion)
	}
	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}
	convertedObjectMap, err := convertRecursive(Object.Object, toSchema, fromVersion)
	if err != nil {
		return nil, statusErrorWithMessage("conversion failed: %v", err.Error())
	}
	convertedObject := &unstructured.Unstructured{
		Object: convertedObjectMap,
	}
	convertedObject.SetAPIVersion(toVersion)
	return convertedObject, statusSucceed()
}
