package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kotsadm/pkg/app"
	"github.com/replicatedhq/kotsadm/pkg/logger"
	"github.com/replicatedhq/kotsadm/pkg/registry"
	"github.com/replicatedhq/kotsadm/pkg/task"
)

type UpdateAppRegistryRequest struct {
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"`
}

type UpdateAppRegistryResponse struct {
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Namespace string `json:"namespace"`
}

func UpdateAppRegistry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "content-type, origin, accept, authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	updateAppRegistryRequest := UpdateAppRegistryRequest{}
	if err := json.NewDecoder(r.Body).Decode(&updateAppRegistryRequest); err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	if err := requireValidSession(w, r); err != nil {
		logger.Error(err)
		return
	}

	currentStatus, err := task.GetTaskStatus("image-rewrite")
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	if currentStatus == "running" {
		logger.Error(errors.New("image-rewrite is already running, not starting a new one"))
		w.WriteHeader(500)
		return
	}

	if err := task.ClearTaskStatus("image-rewrite"); err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	foundApp, err := app.GetFromSlug(mux.Vars(r)["appSlug"])
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	updateAppRegistryResponse := UpdateAppRegistryResponse{
		Hostname:  updateAppRegistryRequest.Hostname,
		Username:  updateAppRegistryRequest.Username,
		Namespace: updateAppRegistryRequest.Namespace,
	}

	// if hostname and namespace have not changed, we don't need to re-push
	registrySettings, err := registry.GetRegistrySettingsForApp(foundApp.ID)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	if registrySettings != nil {
		if registrySettings.Hostname == updateAppRegistryRequest.Hostname {
			if registrySettings.Namespace == updateAppRegistryRequest.Namespace {

				err = registry.UpdateRegistry(foundApp.ID, updateAppRegistryRequest.Hostname, updateAppRegistryRequest.Username, updateAppRegistryRequest.Password, updateAppRegistryRequest.Namespace)
				if err != nil {
					logger.Error(err)
					w.WriteHeader(500)
					return
				}

				JSON(w, 200, updateAppRegistryResponse)
				return
			}
		}
	}

	// in a goroutine, start pushing the images to the remote registry
	// we will let this function return while this happens
	go func() {
		if err := registry.RewriteImages(foundApp.ID, foundApp.CurrentSequence, updateAppRegistryRequest.Hostname, updateAppRegistryRequest.Username, updateAppRegistryRequest.Password,
			updateAppRegistryRequest.Namespace, nil); err != nil {
			logger.Error(err)
			return
		}

		err = registry.UpdateRegistry(foundApp.ID, updateAppRegistryRequest.Hostname, updateAppRegistryRequest.Username, updateAppRegistryRequest.Password, updateAppRegistryRequest.Namespace)
		if err != nil {
			logger.Error(err)
			return
		}
	}()

	JSON(w, 200, updateAppRegistryResponse)
}

func GetAppRegistry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "content-type, origin, accept, authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := requireValidSession(w, r); err != nil {
		logger.Error(err)
		return
	}

}
