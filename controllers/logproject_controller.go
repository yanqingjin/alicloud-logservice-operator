/*


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

package controllers

import (
	"context"
	"fmt"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	logservicev1 "github.com/philips-internal/alicloud-logservice-operator/api/v1"
)

// LogProjectReconciler reconciles a LogProject object
type LogProjectReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=logservice.hsc.philips.com.cn,resources=logprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=logservice.hsc.philips.com.cn,resources=logprojects/status,verbs=get;update;patch

func (r *LogProjectReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("logproject", req.NamespacedName)
	region := os.Getenv("REGION")
	accessKey := os.Getenv("ALICLOUD_ACCESS_KEY")
	secretKey := os.Getenv("ALICLOUD_SECRET_KEY")

	slsClient := sls.CreateNormalInterface(region, accessKey, secretKey, "")

	var logProject logservicev1.LogProject
	if err := r.Get(ctx, req.NamespacedName, &logProject); err != nil {
		log.Error(err, "Unable to fetch Project")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logProjectName := logProject.Spec.Name
	log.Info("Reconciling LogProject: " + logProjectName)
	reconciliationAction := getReconciliationAction(&logProject)
	logServiceFinalizer := "logservice.project.finalizer.hsc.philips.com"

	if reconciliationAction == reconcileAdd {
		isProjectExist, err := slsClient.CheckProjectExist(logProjectName)
		if err != nil {
			log.Error(err, "Unable to check Project status")

			return ctrl.Result{}, err
		}

		if !isProjectExist {
			log.Info("Create Project")
			_, err := slsClient.CreateProject(logProjectName, logProject.Spec.Description)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("project created successfully:" + logProjectName)
		}
		if !containsString(logProject.ObjectMeta.Finalizers, logServiceFinalizer) {
			logProject.ObjectMeta.Finalizers = append(logProject.ObjectMeta.Finalizers, logServiceFinalizer)
			if err := r.Update(context.Background(), &logProject); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else if reconciliationAction == reconcileDelete {
		log.Info("Delete Project")
		if containsString(logProject.ObjectMeta.Finalizers, logServiceFinalizer) {
			err := slsClient.DeleteProject(logProject.Spec.Name)
			if err != nil {
				fmt.Println(err)
				return ctrl.Result{}, err
			}

			logProject.ObjectMeta.Finalizers = removeString(logProject.ObjectMeta.Finalizers, logServiceFinalizer)
			if err := r.Update(context.Background(), &logProject); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *LogProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logservicev1.LogProject{}).
		Complete(r)
}

func getReconciliationAction(logProject *logservicev1.LogProject) ReconciliationAction {
	switch {
	case logProject.ObjectMeta.DeletionTimestamp != nil:
		return reconcileDelete
	default:
		return reconcileAdd
	}
}
