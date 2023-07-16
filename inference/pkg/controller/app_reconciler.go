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

package controller

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	// "google.golang.org/grpc/profiling/service"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/inference/pkg/helper"

	gw "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	AppDefaultRequeueDuration = 3 * time.Second
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	Recorder record.EventRecorder
}

var _ reconcile.Reconciler = (*AppReconciler)(nil)

// Reconcile implements reconcile.Reconciler
// TODO, update application status according service/gateway and httproute status
func (r *AppReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	r.Log.Info("reconciling InferenceApp", "name", request.NamespacedName)

	// Get InferenceApp instance
	var err error
	var appInstance *zetav1.InferenceApp
	if appInstance, err = helper.GetInferenceApp(r.Client, ctx, request.NamespacedName); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if appInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is upated or created,
		if !controllerutil.ContainsFinalizer(appInstance, "finalizer") {
			// so if it does not have our finalizer,
			// then lets add the finalizer and update the object.
			r.Log.Info("Add a finalizer", "finalizer", "todo")
			controllerutil.AddFinalizer(appInstance, "finalizer")
			if err := r.Update(ctx, appInstance); err != nil {
				return ctrl.Result{}, err
			}
		}
		// TODO
		// App is updated or created, reconcile its status, make sure
		// reconcile gateway
		var gateway *gw.Gateway
		if gateway, err = r.reconcileAppGateway(ctx, appInstance); err != nil {
			r.Log.Error(err, "Failed to reconcile gateway for app", "app", appInstance.Name)
			return ctrl.Result{}, err
		}

		// reconcile service
		var service *corev1.Service
		if service, err = r.reconcileAppService(ctx, appInstance); err != nil {
			return ctrl.Result{}, err
		}

		// reconcile httproute
		var route *gw.HTTPRoute
		if route, err = r.reconcileAppGatewayHttpRoute(ctx, appInstance, gateway, service); err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info("Successfully reconcile app with route", "route", route.GetName())
		return ctrl.Result{
			Requeue:      false,
			RequeueAfter: 0,
		}, nil

	} else {
		r.Log.Info("App is being deleted", "DeletionTimestamp", appInstance.ObjectMeta.DeletionTimestamp)
		// TODO
		// delete resources of this app, including
		// gateway
		// service
		// httproute
		// modelctl server will to unload this app from inference server when service is being deleted

		r.Log.Info("Remove the finalizer to let app to be deleted", "finalizer", "finalizer")
		controllerutil.RemoveFinalizer(appInstance, "finalizer")
		err := r.Update(context.TODO(), appInstance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// NewAppReconciler returns a new AppReconciler
func NewAppReconciler(mgr manager.Manager) *AppReconciler {
	return &AppReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("Zeta").WithName("InferenceAppController"),
		Recorder: mgr.GetEventRecorderFor("zeta-inference-app-controller"),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zetav1.InferenceApp{}).
		// Owns(&corev1.Service{}).
		Complete(r)
}

func (r *AppReconciler) reconcileAppAutoScaling(ctx context.Context, app *zetav1.InferenceApp) error {
	// TODO
	// assign or create a gateway (consistent hashing)
	// create gateway http route of app
	return nil
}

// app => gateway => http/grpc route => service
func (r *AppReconciler) reconcileAppGateway(ctx context.Context, app *zetav1.InferenceApp) (gateway *gw.Gateway, err error) {
	// Get InferenceApp Gateway
	// TODO, shared gateway (consistent hashing)
	namespacedname := types.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}
	if gateway, err = helper.GetInferenceAppGateway(r.Client, ctx, namespacedname); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("InferanceApp gateway not found, creating!", "name", namespacedname)
			// create a gateway
			gateway = helper.BuildGatewayForInferenceApp(app)
			if err = helper.CreateResource(r.Client, ctx, gateway); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		updatedGateway := helper.BuildGatewayForInferenceApp(app)
		if !reflect.DeepEqual(updatedGateway.Spec, gateway.Spec) {
			// TODO, use better merge
			gateway.Spec = updatedGateway.Spec
			if err = helper.UpdateResource(r.Client, ctx, gateway); err != nil {
				return nil, err
			}
			r.Log.Info("InferenceApp gateway upated", "namespace", app.Namespace, "app", app.Name)
		}
	}

	r.Log.Info("InferenceApp gateway reconciled", "gateway", gateway.GetGenerateName())
	return gateway, nil
}

// create app gateway http route
func (r *AppReconciler) reconcileAppGatewayHttpRoute(ctx context.Context, app *zetav1.InferenceApp, gateway *gw.Gateway, service *corev1.Service) (route *gw.HTTPRoute, err error) {
	// Get InferenceApp Gateway httproute
	namespacedname := types.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}
	if route, err = helper.GetInferenceAppHttpRoute(r.Client, ctx, namespacedname); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("InferanceApp gateway http route not found, creating!", "name", namespacedname)
			route = helper.BuildHttpRouteForInferenceApp(app, gateway, service)
			if err = helper.CreateResource(r.Client, ctx, route); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		updatedRoute := helper.BuildHttpRouteForInferenceApp(app, gateway, service)
		if !reflect.DeepEqual(updatedRoute.Spec, route.Spec) {
			// TODO, use better merge
			route.Spec = updatedRoute.Spec
			if err = helper.UpdateResource(r.Client, ctx, route); err != nil {
				return nil, err
			}
			r.Log.Info("InferenceApp gateway httproute upated", "namespace", app.Namespace, "app", app.Name)
		}
	}

	r.Log.Info("InferenceApp gateway httproute reconciled", "gateway", gateway.GetGenerateName())
	return route, nil
}

// create service which point to inference instances according model loading status
func (r *AppReconciler) reconcileAppService(ctx context.Context, app *zetav1.InferenceApp) (service *corev1.Service, err error) {
	// Get InferenceApp Gateway
	namespacedname := types.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}
	if service, err = helper.GetInferenceAppService(r.Client, ctx, namespacedname); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("InferenceApp service not found, creating!", "name", namespacedname)
			service = helper.BuildServiceForInferenceApp(app)
			if err = helper.CreateResource(r.Client, ctx, service); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		updatedService := helper.BuildServiceForInferenceApp(app)
		if !reflect.DeepEqual(updatedService.Spec, service.Spec) {
			// TODO use better merge
			service.Spec = updatedService.Spec
			if err = helper.UpdateResource(r.Client, ctx, service); err != nil {
				return nil, err
			}
			r.Log.Info("InferenceApp service upated", "namespace", app.Namespace, "app", app.Name)
		}
	}

	r.Log.Info("InferenceApp service reconciled", "gateway", service.GetGenerateName())
	return service, nil
}
