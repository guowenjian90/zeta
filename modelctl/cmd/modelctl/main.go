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

	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/modelctl/pkg/modelctl/server"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(zetav1.AddToScheme(scheme))
}

func readyCheck(request_ *http.Request) error {
	return nil
}

func healthCheck(request_ *http.Request) error {
	return nil
}

func main() {
	args := &programArgs{}

	parseArgs(args)

	initLogger(args)

	ctrl.Log.Info("Initializing zeta modelctl server", "args", args.probeAddr, "enable election", args.enableLeaderElection)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Namespace:              args.nameSpace,
		HealthProbeBindAddress: args.probeAddr,
		LeaderElection:         args.enableLeaderElection,
		LeaderElectionID:       "zeta-modelctl-server",
		MetricsBindAddress:     args.metricsAddr,
		Port:                   9443,
		Scheme:                 scheme,
	})
	if err != nil {
		ctrl.Log.Error(err, "Unable to new model controller manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthCheck); err != nil {
		ctrl.Log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", readyCheck); err != nil {
		ctrl.Log.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	if modelctlServer, err := server.NewServer(mgr); err != nil {
		ctrl.Log.Error(err, "Unable to initialize modelctl server")
		os.Exit(1)
	} else {
		ctrl.Log.Info("Starting ModelCtl server")
		ctx := ctrl.SetupSignalHandler()
		if err := modelctlServer.Run(ctx); err != nil {
			ctrl.Log.Error(err, "Faied to start ModelCtl server")
			os.Exit(1)
		}
		ctrl.Log.Info("ModelCtl Server started, looping for exit signal")
		if err := mgr.Start(ctx); err != nil {
			ctrl.Log.Error(err, "Failed to start ModelCtl controller manager")
			os.Exit(1)
		}
	}

}

func parseArgs(args *programArgs) {
	flag.BoolVar(&args.help, "help", false, "Show the help information.")
	flag.StringVar(&args.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&args.probeAddr, "probe-addr", ":8082", "The address the probe endpoint binds to.")
	flag.StringVar(&args.nameSpace, "namespace", "zeta", "The namespace of managed resouces.")
	flag.BoolVar(&args.enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	opts := k8szap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
}

func initLogger(args *programArgs) {
	opts := &k8szap.Options{}
	if args.logFile != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   args.logFile,
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
