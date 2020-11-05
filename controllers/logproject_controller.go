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
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

	logProject := &logservicev1.LogProject{}
	if err := r.Get(ctx, req.NamespacedName, logProject); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("LogProject resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch LogProject")

		return ctrl.Result{}, err
	}

	logProjectName := logProject.Spec.Name
	log.Info("Reconciling LogProject=" + logProjectName)
	logServiceFinalizer := "logservice.project.finalizer.hsc.philips.com"

	isProjectExist, err := slsClient.CheckProjectExist(logProjectName)
	if err != nil {
		log.Error(err, "Unable to check Project status")
		return ctrl.Result{}, err
	}

	if !isProjectExist {
		log.Info("Create New Project")
		_, err := slsClient.CreateProject(logProjectName, logProject.Spec.Description)
		if err != nil {
			return ctrl.Result{}, err
		}
		logProject.Status.Spec = logProject.Spec
		err = r.Status().Update(ctx, logProject)
		if err != nil {
			log.Error(err, "Failed to update LogService status")
			return ctrl.Result{}, err
		}
		log.Info("LogProject created successfully:" + logProjectName)
		log.Info("Append Finalizer")
		if !containsString(logProject.ObjectMeta.Finalizers, logServiceFinalizer) {
			logProject.ObjectMeta.Finalizers = append(logProject.ObjectMeta.Finalizers, logServiceFinalizer)
			if err = r.Update(context.Background(), logProject); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if logProject.ObjectMeta.DeletionTimestamp != nil {
		log.Info("Delete Project")
		if containsString(logProject.ObjectMeta.Finalizers, logServiceFinalizer) {
			err := slsClient.DeleteProject(logProjectName)
			if err != nil {
				log.Error(err, "Failed to delete LogService")
				return ctrl.Result{}, err
			}

			logProject.ObjectMeta.Finalizers = removeString(logProject.ObjectMeta.Finalizers, logServiceFinalizer)
			if err = r.Update(context.Background(), logProject); err != nil {
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
