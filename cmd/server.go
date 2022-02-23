package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tolson-vkn/pifrost/provider"
	"github.com/tolson-vkn/pifrost/watcher"
)

var (
	insecure    bool
	autoIngress bool
	piHoleHost  string
	ingressEIP  string
	piHoleToken string
	kubeconfig  string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start server",
	Long:  `Start ExternalDNS pihole server daemon`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(piHoleHost) == 0 {
			logrus.Fatal("Need to specify: --pihole-host")
		}

		if len(piHoleToken) == 0 {
			logrus.Fatal("Need to specify: --pihole-token")
		}

		kconfig := new(rest.Config)
		var err error
		if len(kubeconfig) == 0 {
			kconfig, err = rest.InClusterConfig()
			if err != nil {
				logrus.Fatal("Could not get in cluster config: %s", err)
			}
		} else {
			kconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				logrus.Fatal("Could not get out of cluster config: %s", err)
			}
		}

		dnsProvider, err := provider.InitDNSProvider(
			insecure,
			piHoleHost,
			piHoleToken,
		)
		if err != nil {
			logrus.Fatalf("Could not initialize DNS provider: %s", err)
		}

		err = dnsProvider.ValidateProvider()
		if err != nil {
			logrus.Fatalf("Could not validate DNS provider: %s", err)
		}

		watcher.Watch(dnsProvider, kconfig, autoIngress, ingressEIP)
	},
}

func init() {
	serverCmd.Flags().BoolVar(&insecure, "insecure", false, "communicate over http:// (default: https://)")
	serverCmd.Flags().StringVar(&piHoleHost, "pihole-host", "", "hostname or IP of pihole instance")
	serverCmd.Flags().StringVar(&piHoleToken, "pihole-token", "", "API token for pihole")
	serverCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to kubeconfig (default: in cluster config)")
	serverCmd.Flags().BoolVar(&autoIngress, "ingress-auto", false, "do not require annotation on ingress resources (default: false)")
	serverCmd.Flags().StringVar(&ingressEIP, "ingress-externalip", "", "force use of provided external ip (default: use ingress external ip)")
}
