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

package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	gateway "sigs.k8s.io/gateway-api/apis/v1beta1"

	// +kubebuilder:scaffold:imports

	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/inference/pkg/controller"
)

var (
	scheme = runtime.NewScheme()
)

func readyCheck(request_ *http.Request) error {
	return nil
}

func healthCheck(request_ *http.Request) error {
	return nil
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gateway.AddToScheme(scheme))
	utilruntime.Must(zetav1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	cliArgs := &programArgs{}

	parseArgs(cliArgs)

	initLogger(cliArgs)

	ctrl.Log.Info("Initializing inference application controller")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Namespace:              cliArgs.nameSpace,
		HealthProbeBindAddress: cliArgs.probeAddr,
		LeaderElection:         cliArgs.enableLeaderElection,
		LeaderElectionID:       "zeta-inference-leader",
		MetricsBindAddress:     cliArgs.metricsAddr,
		Port:                   9443,
		Scheme:                 scheme,
	})
	if err != nil {
		ctrl.Log.Error(err, "Unable to new controller manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthCheck); err != nil {
		ctrl.Log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", readyCheck); err != nil {
		ctrl.Log.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	if err = controller.NewAppReconciler(mgr).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "Unable to create inference application reconciler")
		os.Exit(1)
	}

	ctrl.Log.Info("Starting inference application controller manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "Failed to start controller manager")
		os.Exit(1)
	}
}

func parseArgs(args *programArgs) {
	flag.BoolVar(&args.help, "help", false, "Show the help information.")
	flag.StringVar(&args.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&args.probeAddr, "health-probe-bind-address", ":8082", "The address the probe endpoint binds to.")
	flag.StringVar(&args.nameSpace, "namespace", "zeta", "The namespace of managed resouces.")
	flag.BoolVar(&args.enableLeaderElection, "enable-leader-election", true, "Enable leader election. there is only one active controller if true.")

	opts := k8szap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
}

func initLogger(cliArgs *programArgs) {
	opts := &k8szap.Options{}
	if cliArgs.logFile != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   cliArgs.logFile,
			MaxSize:    500, // megabytes
			MaxBackups: 10,  // files
			MaxAge:     30,  // days
		}

		pe := zap.NewProductionEncoderConfig()
		pe.EncodeTime = zapcore.ISO8601TimeEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(pe)

		k8sLogger := k8szap.NewRaw(k8szap.UseFlagOptions(opts))
		zapOpts := append(opts.ZapOpts, zap.AddCallerSkip(1))
		combineLogger := zap.New(zapcore.NewTee(
			k8sLogger.Core(),
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(fileWriter), zap.InfoLevel),
		)).WithOptions(zapOpts...)
		combineLoggerR := zapr.NewLogger(combineLogger)

		ctrl.SetLogger(combineLoggerR)
	} else {
		ctrl.SetLogger(k8szap.New(k8szap.UseFlagOptions(opts)))
	}
}

type programArgs struct {
	help                 bool
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	nameSpace            string
	logFile              string
}
