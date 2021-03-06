package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type providers struct {
	microk8s      bool
	dockerDesktop bool
	eks           bool
	gke           bool
	digitalOcean  bool
}

type Provider int

const (
	unknown       Provider = iota
	microk8s      Provider = iota
	dockerDesktop Provider = iota
	eks           Provider = iota
	gke           Provider = iota
	digitalOcean  Provider = iota
)

func analyzeDistribution(analyzer *troubleshootv1beta1.Distribution, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents("cluster-resources/nodes.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes []corev1.Node
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	foundProviders := providers{}

	for _, node := range nodes {
		for k, v := range node.ObjectMeta.Labels {
			if k == "microk8s.io/cluster" && v == "true" {
				foundProviders.microk8s = true
			}
		}

		if node.Status.NodeInfo.OSImage == "Docker Desktop" {
			foundProviders.dockerDesktop = true
		}

		if strings.HasPrefix(node.Spec.ProviderID, "digitalocean:") {
			foundProviders.digitalOcean = true
		}
		if strings.HasPrefix(node.Spec.ProviderID, "aws:") {
			foundProviders.eks = true
		}
	}

	result := &AnalyzeResult{
		Title: "Kubernetes Distribution",
	}

	// ordering is important for passthrough
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Fail.When, foundProviders)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Warn.When, foundProviders)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			isMatch, err := compareDistributionConditionalToActual(outcome.Pass.When, foundProviders)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare distribution conditional")
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

		}
	}

	return result, nil
}

func compareDistributionConditionalToActual(conditional string, actual providers) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	// we can make this a lot more flexible
	if len(parts) == 1 {
		parts = []string{
			"=",
			parts[0],
		}
	}

	if len(parts) != 2 {
		return false, errors.New("unable to parse conditional")
	}

	normalizedName := mustNormalizeDistributionName(parts[1])

	if normalizedName == unknown {
		return false, nil
	}

	isMatch := false
	switch normalizedName {
	case microk8s:
		isMatch = actual.microk8s
	case dockerDesktop:
		isMatch = actual.dockerDesktop
	case eks:
		isMatch = actual.eks
	case gke:
		isMatch = actual.gke
	case digitalOcean:
		isMatch = actual.digitalOcean
	}

	switch parts[0] {
	case "=", "==", "===":
		return isMatch, nil
	case "!=", "!==":
		return !isMatch, nil
	}

	return false, nil
}

func mustNormalizeDistributionName(raw string) Provider {
	switch strings.ReplaceAll(strings.TrimSpace(strings.ToLower(raw)), "-", "") {
	case "microk8s":
		return microk8s
	case "dockerdesktop":
		return dockerDesktop
	case "eks":
		return eks
	case "gke":
		return gke
	case "digitalocean":
		return digitalOcean
	}

	return unknown
}
