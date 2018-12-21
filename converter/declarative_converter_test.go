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
	"reflect"
	"testing"

	"github.com/mbohlool/kcrdc/types"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type testSchemaAccess struct {
	schemas map[string]*v1beta1.JSONSchemaProps
}

var _ types.CRDSchemaAccess = &testSchemaAccess{}

func (t *testSchemaAccess) GetSchema(apiVersion, kind string) *v1beta1.JSONSchemaProps {
	return t.schemas[apiVersion+"/"+kind]
}

func TestRenameConversion(t *testing.T) {
	testCases := []struct {
		schemas         map[string]*v1beta1.JSONSchemaProps
		input, expected map[string]interface{}
	}{
		{
			schemas: map[string]*v1beta1.JSONSchemaProps{
				"example.com/v1/foo": &v1beta1.JSONSchemaProps{
					Properties: map[string]v1beta1.JSONSchemaProps{
						"server": v1beta1.JSONSchemaProps{
							Type:        "string",
							Description: "+conversion:example.com/v2,rename,host",
						},
					},
				},
				"example.com/v2/foo": &v1beta1.JSONSchemaProps{
					Properties: map[string]v1beta1.JSONSchemaProps{
						"host": v1beta1.JSONSchemaProps{
							Type:        "string",
							Description: "+conversion:example.com/v1,rename,server",
						},
					},
				},
			},
			input: map[string]interface{}{
				"server":     "http://example.com",
				"kind":       "foo",
				"apiVersion": "example.com/v1",
				"metadata": map[string]interface{}{
					"name": "myfoo",
				},
			},
			expected: map[string]interface{}{
				"host":       "http://example.com",
				"kind":       "foo",
				"apiVersion": "example.com/v2",
				"metadata": map[string]interface{}{
					"name": "myfoo",
				},
			},
		},
		{
			schemas: map[string]*v1beta1.JSONSchemaProps{
				"example.com/v1/foo": &v1beta1.JSONSchemaProps{
					Properties: map[string]v1beta1.JSONSchemaProps{
						"server": v1beta1.JSONSchemaProps{
							Type:        "string",
							Description: "+conversion:example.com/v2,rename,host",
						},
					},
				},
				"example.com/v2/foo": &v1beta1.JSONSchemaProps{
					Properties: map[string]v1beta1.JSONSchemaProps{
						"host": v1beta1.JSONSchemaProps{
							Type:        "string",
							Description: "+conversion:example.com/v1,rename,server",
						},
					},
				},
			},
			input: map[string]interface{}{
				"host":       "http://example.com",
				"kind":       "foo",
				"apiVersion": "example.com/v2",
				"metadata": map[string]interface{}{
					"name": "myfoo",
				},
			},
			expected: map[string]interface{}{
				"server":     "http://example.com",
				"kind":       "foo",
				"apiVersion": "example.com/v1",
				"metadata": map[string]interface{}{
					"name": "myfoo",
				},
			},
		},
	}

	for _, ts := range testCases {
		testSchemaAccess := testSchemaAccess{
			schemas: ts.schemas,
		}
		input := &unstructured.Unstructured{
			Object: ts.input,
		}
		expected := &unstructured.Unstructured{
			Object: ts.expected,
		}

		converter := &DeclarativeConverter{&testSchemaAccess}

		converted, status := converter.convert(input, ts.expected["apiVersion"].(string))
		if status.Status != v1.StatusSuccess {
			t.Fatalf("conversion failed: %v", status)
		}
		if e, a := expected, converted; !reflect.DeepEqual(e, a) {
			t.Fatalf("expected = %v, actual = %v", e, a)
		}
	}
}
