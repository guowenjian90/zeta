/*
Copyright 2022 The CentaurusAI Authors.

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
package model

import (
	"CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/modelctl/pkg/types"
	"encoding/json"
	"regexp"

	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	v1 "k8s.io/api/core/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

const (
	DefaultModelRepositoryRootPath = "/var/zeta/model_repository"
)

// model_repository
// ├── text_detection
// │   ├── 1
// │   │   └── model.onnx
// │   └── config.pbtxt
// └── text_recognition
//     ├── 1
//     │   └── model.onnx
//     └── config.pbtxt

type ModelLoader struct {
	endpoint     string
	rootPath     string
	modelConfigs map[ktypes.UID]*ModelConfiguration
}

func NewModelLoader(ip string) *ModelLoader {
	endpoint := "http://localhost:8000"
	if len(ip) > 0 {
		endpoint = fmt.Sprintf("http://%s:8000", ip)
	}
	if strings.HasSuffix(endpoint, "/") {
		endpoint = strings.TrimSuffix(endpoint, "/")
	}
	return &ModelLoader{
		endpoint:     endpoint,
		rootPath:     DefaultModelRepositoryRootPath,
		modelConfigs: map[ktypes.UID]*ModelConfiguration{},
	}
}

// Path: model config path, also serve as model path, e.g. /var/zeta/model_repository/text_detection
// Content: config.pbtxt, model config file content
//
//	name: "text_detection"
//	backend: "onnxruntime"
//	max_batch_size : 0
//	input [
//	 {
//	   name: "input_images:0"
//	   data_type: TYPE_FP32
//	   dims: [ -1, -1, -1, 3 ]
//	 }
//	]
//	output [
//	 {
//	   name: "feature_fusion/Conv_7/Sigmoid:0"
//	   data_type: TYPE_FP32
//	   dims: [ -1, -1, -1, 1 ]
//	 }
//	]
//	output [
//	 {
//	   name: "feature_fusion/concat_3:0"
//	   data_type: TYPE_FP32
//	   dims: [ -1, -1, -1, 5 ]
//	 }
//	]
type ModelConfiguration struct {
	Uid          ktypes.UID
	Name         string
	Namespace    string
	OsPath       string
	Version      string
	CtrlEndpoint string
	Content      string
	Model        *v1alpha1.Model
}

func (mr *ModelLoader) init() error {
	if _, err := os.Stat(mr.rootPath); os.IsNotExist(err) {
		os.MkdirAll(mr.rootPath, fs.ModeDir.Perm())
	} else if err != nil {
		return err
	}

	// find existing model
	_, err := mr.ListModels()
	return err
}

type TritionModel struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	State   string `json:"state,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/index
// [
// {"name":"text_detection","version":"1","state":"UNAVAILABLE","reason":"unloaded"},
// {"name":"text_recognition","version":"1","state":"READY"}
// {"name":"text_recognition_2"}
// ]
func (mr *ModelLoader) ListModels() ([]*types.Model, error) {
	p := mr.endpoint + "/v2/repository/index"
	resp, err := http.Post(p, "", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("List models failed with error code, %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	var tritonModels = []TritionModel{}
	if err := json.NewDecoder(resp.Body).Decode(&tritonModels); err != nil {
		return nil, err
	}

	loadedModelIds := map[string]bool{}
	models := make([]*types.Model, len(tritonModels))
	for k, v := range tritonModels {
		loadedModelIds[v.Name] = true
		model := &types.Model{
			Uid:     ktypes.UID(v.Name),
			Version: v.Version,
			ModelState: &types.ModelState{
				ServingState: types.ServingState(v.State),
				ModelStatistics: types.ModelStatistics{
					ResourceUsage:    map[v1.ResourceName]types.ResourceUsage{},
					RequestStatistic: types.RequestStatistic{},
				}, // TODO, capture and save model serve statistics
			},
		}
		mc := mr.modelConfigs[ktypes.UID(v.Name)]
		if mc != nil {
			model.Name = mc.Name
			model.Namespace = mc.Namespace
			model.APIModel = mc.Model
		}
		models[k] = model
	}

	// fill model just created and not loaded yet
	for k, v := range mr.modelConfigs {
		if _, found := loadedModelIds[string(k)]; !found {
			model := &types.Model{
				Uid:       k,
				Version:   v.Version,
				Name:      v.Name,
				Namespace: v.Namespace,
				ModelState: &types.ModelState{
					ServingState: types.ServingStateCreated,
					ModelStatistics: types.ModelStatistics{
						ResourceUsage:    map[v1.ResourceName]types.ResourceUsage{},
						RequestStatistic: types.RequestStatistic{},
					}, // TODO, capture and save model serve statistics
				},
				APIModel: v.Model,
			}
			models = append(models, model)
		}
	}

	return models, nil
}

// curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/models/text_detection/load
func (mr *ModelLoader) LoadModel(model types.Model, lazy bool) error {
	if existing, found := mr.modelConfigs[model.Uid]; found && existing.Version == model.Version {
		// model of expected version already build, skip
		return nil
	} else {
		config, err := mr.buildModel(model, found)
		if err != nil {
			return err
		}
		mr.modelConfigs[model.Uid] = config

		p := config.CtrlEndpoint + "/load"
		res, err := http.Post(p, "", nil)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			return fmt.Errorf("Load Model %s failed with status code %d", model.Uid, res.StatusCode)
		}
	}

	return nil
}

// curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/models/text_detection/unload
func (mr *ModelLoader) UnloadModel(model types.Model) error {
	if config, found := mr.modelConfigs[model.Uid]; found {
		p := config.CtrlEndpoint + "/unload"
		res, err := http.Post(p, "", nil)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			return fmt.Errorf("Unload Model %s failed with status code %d", model.Uid, res.StatusCode)
		}
	} else {
		return fmt.Errorf("Model %s is not loaded", model.Uid)
	}
	return nil
}

func (mr *ModelLoader) buildModel(model types.Model, forceReplace bool) (*ModelConfiguration, error) {
	hostPath := path.Join(mr.rootPath, string(model.Uid))
	if _, err := os.Stat(hostPath); os.IsNotExist(err) {
		if err := os.MkdirAll(hostPath, 0744); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		// model already exist, should not happen, do not overwrite, model has to be unloaded before load it again
		if forceReplace {
			return nil, fmt.Errorf("Model %s is already loaded, unload it and load it again for reload", model.Uid)
		}
	}

	url := model.APIModel.Url
	if remoteReader, err := getModelRepository(url); err != nil {
		return nil, err
	} else {
		if err = remoteReader.Download(string(model.Name), url, hostPath); err != nil {
			return nil, err
		}
	}

	// TODO, this should be better
	// a triton server can host multiple models from multiple customers, they could use same name,
	// triton server require model name is unique and same as inference request path,
	// and api server give each model a uid, use model's uid as path, in this case, it mitigate conflict with other models with same name,
	// and model app gateway need to configured to use id as api route path also
	// use Uid as name, name should be unique in a triton server
	configPath := path.Join(hostPath, "config.pbtxt")
	if content, err := os.ReadFile(configPath); err != nil {
		return nil, err
	} else {
		contentString := string(content)
		lines := strings.Split(contentString, "\n")
		for k, v := range lines {
			if match, err := regexp.MatchString("name: \".*\"", v); match && err == nil {
				lines[k] = fmt.Sprintf("name: \"%s\"", string(model.Uid))
				break
			}
		}
		content = []byte(strings.Join(lines, "\n"))
		if err = os.WriteFile(configPath, content, 0644); err != nil {
			return nil, err
		}
		config := &ModelConfiguration{
			Uid:          model.Uid,
			Name:         model.Name,
			Namespace:    model.Namespace,
			Version:      model.Version,
			OsPath:       hostPath,
			CtrlEndpoint: mr.endpoint + "/v2/repository/models/" + string(model.Uid),
			Content:      contentString,
		}
		return config, nil
	}
}

func getModelRepository(url string) (ModelRepository, error) {
	switch url {
	// TODO, support multiple protocol, e.g. s3
	default:
		return &LocalRepository{
			localPath: "/model_repository",
		}, nil
	}
}

type ModelRepository interface {
	Download(name, url, osPath string) error
}
