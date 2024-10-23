package integration

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelector(t *testing.T) {
	sysCfg := baseSysCfg(t)
	initTest(t, sysCfg)

	const (
		commonLabelKey     = "test-label-key"
		m0m1CommonLabelVal = "test-label-val-found"
		m2CommonLabelVal   = "test-label-val-not-found"

		m0OnlyLabelKey = "m0-only-label-key"
		m0OnlyLabelVal = "m0-only-label-val"
		m1OnlyLabelKey = "m1-only-label-key"
		m1OnlyLabelVal = "m1-only-label-val"
	)
	// Model with an active backend to send requests to.
	m0 := modelForTest(t)
	m0.Name = m0.Name + "0"
	m0.Labels[commonLabelKey] = m0m1CommonLabelVal
	m0.Labels[m0OnlyLabelKey] = m0OnlyLabelVal
	require.NoError(t, testK8sClient.Create(testCtx, m0))

	testModelBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serving request from testBackend")
		w.WriteHeader(200)
	}))
	updateModelWithBackend(t, m0, testModelBackend)

	// Update with MinReplicas after setting annotation to trigger Pod creation
	// with annotations pointing to test backend.
	updateModel(t, m0, func() {
		m0.Spec.MinReplicas = 1
	}, "Set MinReplicas to 1")
	requireModelPods(t, m0, 1, "Min replica Pod should be created", 5*time.Second)
	markAllModelPodsReady(t, m0)

	//logPods(t)

	inferenceCases := []struct {
		name            string
		modelName       string
		selectorHeaders []string
		expCode         int
		expBodyContains string
	}{
		{
			name:            "non existent model",
			modelName:       "does-not-exist",
			selectorHeaders: []string{commonLabelKey + "=" + m0m1CommonLabelVal},
			expCode:         http.StatusNotFound,
			expBodyContains: "model not found",
		},
		{
			name:            "existant model no match",
			modelName:       m0.Name,
			selectorHeaders: []string{commonLabelKey + "=" + m2CommonLabelVal},
			expCode:         http.StatusNotFound,
			expBodyContains: "model not found",
		},
		{
			name:            "existant model 1/2 match single header",
			modelName:       m0.Name,
			selectorHeaders: []string{commonLabelKey + "=" + m2CommonLabelVal + "," + m1OnlyLabelKey + "=" + m1OnlyLabelVal},
			expCode:         http.StatusNotFound,
			expBodyContains: "model not found",
		},
		{
			name:      "existant model 1/2 match separate headers",
			modelName: m0.Name,
			selectorHeaders: []string{
				commonLabelKey + "=" + m2CommonLabelVal,
				m1OnlyLabelKey + "=" + m1OnlyLabelVal,
			},
			expCode:         http.StatusNotFound,
			expBodyContains: "model not found",
		},
		{
			name:            "model exists 2/2 labels match single header",
			modelName:       m0.Name,
			selectorHeaders: []string{commonLabelKey + "=" + m0m1CommonLabelVal + "," + m0OnlyLabelKey + "=" + m0OnlyLabelVal},
			expCode:         http.StatusOK,
		},
		{
			// `AND` logic should be used.
			// This is important because if `OR` logic were used it would open up a possible vulerability: if the headers that an end-user specified were proxied with `OR` logic it would allow users to circumvent and proxy-enforced selectors.
			name:      "model exists 2/2 labels match separate headers",
			modelName: m0.Name,
			selectorHeaders: []string{
				commonLabelKey + "=" + m0m1CommonLabelVal,
				m0OnlyLabelKey + "=" + m0OnlyLabelVal,
			},
			expCode: http.StatusOK,
		},
		{
			name:            "model exists 1/1 labels match",
			modelName:       m0.Name,
			selectorHeaders: []string{commonLabelKey + "=" + m0m1CommonLabelVal},
			expCode:         http.StatusOK,
		},
		{
			name:            "model exists 1/1 labels match in",
			modelName:       m0.Name,
			selectorHeaders: []string{fmt.Sprintf("%s in (%s)", m0OnlyLabelKey, m0OnlyLabelVal)},
			expCode:         http.StatusOK,
		},
	}
	for _, c := range inferenceCases {
		t.Run("inference "+c.name, func(t *testing.T) {
			t.Parallel()
			sendOpenAIInferenceRequest(t,
				c.modelName, c.selectorHeaders,
				c.expCode, c.expBodyContains,
				c.name)
		})
	}

	// Secondary model for listing.
	m1 := modelForTest(t)
	m1.Name = m1.Name + "1"
	m1.Labels[commonLabelKey] = m0m1CommonLabelVal
	m1.Labels[m1OnlyLabelKey] = m1OnlyLabelVal
	require.NoError(t, testK8sClient.Create(testCtx, m1))

	// Third model for filtering out.
	m2 := modelForTest(t)
	m2.Name = m2.Name + "2"
	m2.Labels[commonLabelKey] = m2CommonLabelVal
	require.NoError(t, testK8sClient.Create(testCtx, m2))

	// Wait for cache to sync.
	time.Sleep(time.Second)

	listTestCases := []struct {
		name            string
		selectorHeaders []string
		expLen          int
		expModels       []string
	}{
		{
			name:            "one selector two models",
			selectorHeaders: []string{commonLabelKey + "=" + m0m1CommonLabelVal},
			expLen:          2,
			expModels:       []string{m0.Name, m1.Name},
		},
		{
			name: "two selectors one header one model",
			selectorHeaders: []string{
				commonLabelKey + "=" + m0m1CommonLabelVal + "," +
					m0OnlyLabelKey + "=" + m0OnlyLabelVal,
			},
			expLen:    1,
			expModels: []string{m0.Name},
		},
		{
			name: "two selectors two headers one model",
			selectorHeaders: []string{
				commonLabelKey + "=" + m0m1CommonLabelVal,
				m0OnlyLabelKey + "=" + m0OnlyLabelVal,
			},
			expLen:    1,
			expModels: []string{m0.Name},
		},
		{
			name:            "other model",
			selectorHeaders: []string{commonLabelKey + "=" + m2CommonLabelVal},
			expLen:          1,
			expModels:       []string{m2.Name},
		},
		{
			name: "single in selector all three models",
			selectorHeaders: []string{
				fmt.Sprintf("%s in (%s, %s)", commonLabelKey, m0m1CommonLabelVal, m2CommonLabelVal),
			},
			expLen:    3,
			expModels: []string{m0.Name, m1.Name, m2.Name},
		},
	}
	for _, c := range listTestCases {
		t.Run("list "+c.name, func(t *testing.T) {
			t.Parallel()
			list := sendOpenAIListModelsRequest(t, c.selectorHeaders, http.StatusOK, c.name)
			require.Len(t, list, c.expLen)
			ids := make([]string, len(list))
			for i, m := range list {
				ids[i] = m.ID
			}
			require.ElementsMatch(t, c.expModels, ids)
		})
	}
}
