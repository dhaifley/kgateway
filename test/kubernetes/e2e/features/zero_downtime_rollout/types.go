package zero_downtime_rollout

import (
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	setup = base.TestCase{
		Manifests: []string{routeWithServiceManifest},
	}

	testCases = map[string]*base.TestCase{
		"TestZeroDowntimeRollout": {
			Manifests: []string{heyManifest, defaults.CurlPodManifest},
		},
	}
)
