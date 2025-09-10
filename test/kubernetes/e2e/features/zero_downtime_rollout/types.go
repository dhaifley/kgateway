package zero_downtime_rollout

import (
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kgateway-dev/kgateway/v2/pkg/utils/fsutils"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e/defaults"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e/tests/base"
)

var (
	routeWithServiceManifest = filepath.Join(fsutils.MustGetThisDir(), "testdata", "route-with-service.yaml")
	heyManifest              = filepath.Join(fsutils.MustGetThisDir(), "testdata", "hey.yaml")

	proxyObjectMeta = metav1.ObjectMeta{
		Name:      "gw",
		Namespace: "default",
	}
	proxyDeployment = &appsv1.Deployment{ObjectMeta: proxyObjectMeta}
	proxyService    = &corev1.Service{ObjectMeta: proxyObjectMeta}

	heyPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hey",
			Namespace: "hey",
		},
	}

	nginxPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
		},
	}

	exampleService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-svc",
			Namespace: "default",
		},
	}

	exampleRoute = &apiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-route",
			Namespace: "default",
		},
	}

	setup = base.TestCase{
		Manifests: []string{routeWithServiceManifest},
		Resources: []client.Object{proxyDeployment, proxyService, exampleService, nginxPod, exampleRoute},
	}

	testCases = map[string]base.TestCase{
		"TestZeroDowntimeRollout": {
			Manifests: []string{heyManifest, defaults.CurlPodManifest},
			Resources: []client.Object{heyPod, defaults.CurlPod},
		},
	}
)
