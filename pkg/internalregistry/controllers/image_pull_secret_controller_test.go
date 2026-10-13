package controllers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestRefreshThresholdTime(t *testing.T) {
	ttime := func(d time.Duration) time.Time {
		return time.Time{}.Add(d)
	}
	testCases := []struct {
		name string
		nbf  time.Time
		exp  time.Time
		want time.Time
	}{
		{name: "happya", nbf: ttime(0), exp: ttime(10000 * time.Hour), want: ttime(6000 * time.Hour)},
		{name: "happyb", nbf: ttime(0), exp: ttime(10000 * time.Minute), want: ttime(6000 * time.Minute)},
		{name: "happyc", nbf: ttime(0), exp: ttime(10000 * time.Second), want: ttime(6000 * time.Second)},
		{name: "invalid", nbf: ttime(10), exp: ttime(5), want: ttime(0)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refreshThresholdTime(tc.nbf, tc.exp); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("refreshThresholdTime() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsSecretRefreshNeededA(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		secrets         v1.SecretLister
		serviceAccounts v1.ServiceAccountLister
		cacheSyncs      []cache.InformerSynced
		queue           workqueue.RateLimitingInterface
		urls            *atomic.Pointer[[]string]
		urlsC           chan []string
		kids            *atomic.Pointer[[]string]
		kidsC           chan []string
	}
	type args struct {
		secret *corev1.Secret
		urls   []string
		kids   []string
		now    time.Time
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  *time.Time
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := isSecretRefreshNeeded(tt.args.secret, tt.args.urls, tt.args.kids, tt.args.now)
			if got != tt.want {
				t.Errorf("isSecretRefreshNeeded() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("isSecretRefreshNeeded() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func secret(opts ...func(*corev1.Secret)) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Type: corev1.SecretTypeDockercfg,
	}
	for _, f := range opts {
		f(s)
	}
	return s
}

func withType(t corev1.SecretType) func(*corev1.Secret) {
	return func(s *corev1.Secret) {
		s.Type = t
	}
}

func withDockerConfig(opts ...func(map[string]map[string]string)) func(*corev1.Secret) {
	cfg := map[string]map[string]string{}
	return func(s *corev1.Secret) {
		for _, opt := range opts {
			opt(cfg)
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal docker config: %v", err))
		}
		fmt.Println(string(data))
		if s.Data == nil {
			s.Data = make(map[string][]byte)
		}
		s.Data[corev1.DockerConfigKey] = data
	}
}

func withCredentials(url, auth string) func(map[string]map[string]string) {
	return func(cfg map[string]map[string]string) {
		cfg[url] = map[string]string{"auth": auth}
	}
}

func tokenAuth(kid string, nbf, exp time.Time) string {
	h, err := json.Marshal(map[string]interface{}{"alg": "none", "kid": kid})
	if err != nil {
		panic(err)
	}
	p, err := json.Marshal(map[string]interface{}{"nbf": nbf.Unix(), "exp": exp.Unix()})
	if err != nil {
		panic(err)
	}
	t := fmt.Sprintf("%s.%s.", base64.RawURLEncoding.EncodeToString(h), base64.RawURLEncoding.EncodeToString(p))
	return base64.StdEncoding.EncodeToString([]byte("<token>:" + t))
}

func TestIsSecretRefreshNeeded(t *testing.T) {
	now := time.Now().Round(time.Second)
	ts := func(d time.Duration) time.Time {
		return now.Add(d)
	}
	tsPtr := func(d time.Duration) *time.Time {
		p := ts(d)
		return &p
	}
	tests := []struct {
		name           string
		secret         *corev1.Secret
		urls           []string
		kids           []string
		wantRefreshNow bool
		wantRefreshAt  *time.Time
	}{
		{
			name: "wrong secret type",
			urls: []string{"registry.internal"},
			kids: []string{"kid1"},
			secret: secret(
				withType(corev1.SecretTypeDockerConfigJson),
				withDockerConfig(
					withCredentials("registry.internal", tokenAuth("kid1", ts(-5*time.Minute), ts(30*time.Minute))),
				),
			),
			wantRefreshNow: true,
		},
		{
			name:           "missing docker config key",
			urls:           []string{"url1"},
			kids:           []string{"kid1"},
			secret:         secret(),
			wantRefreshNow: true,
		},
		{
			name: "malformed docker config key value",
			urls: []string{"url1"},
			kids: []string{"kid1"},
			secret: secret(func(s *corev1.Secret) {
				s.Data = map[string][]byte{corev1.DockerConfigKey: []byte("not.a.valid.dockercfg")}
			}),
			wantRefreshNow: true,
		},
		{
			name: "incorrect number of entries",
			urls: []string{"url1", "url2"},
			kids: []string{"kid1"},
			secret: secret(
				withDockerConfig(
					withCredentials("url1", tokenAuth("kid1", ts(-5*time.Minute), ts(30*time.Minute))),
				)),
			wantRefreshNow: true,
		},
		{
			name: "incorrect list of entries",
			urls: []string{"url1", "url2"},
			kids: []string{"kid1"},
			secret: secret(
				withDockerConfig(
					withCredentials("url1", tokenAuth("kid1", ts(-5*time.Minute), ts(30*time.Minute))),
					withCredentials("url3", tokenAuth("kid1", ts(-5*time.Minute), ts(30*time.Minute))),
				)),
			wantRefreshNow: true,
		},
		{
			name: "malformed token",
			urls: []string{"url1"},
			kids: []string{"kid1"},
			secret: secret(
				withDockerConfig(
					withCredentials("url1", base64.StdEncoding.EncodeToString([]byte("<token>:not.a.valid.token"))),
				),
			),
			wantRefreshNow: true,
		},
		{
			name: "token signed with wrong kid",
			urls: []string{"url1"},
			kids: []string{"kid1"}, // expected kid is "kid1"
			secret: secret(
				withDockerConfig(
					withCredentials("url1", tokenAuth("kid2", ts(-5*time.Minute), ts(30*time.Minute))),
				),
			),
			wantRefreshNow: true,
		},
		{
			name: "valid secret, not near expiry",
			urls: []string{"url1"},
			kids: []string{"kid1"},
			secret: secret(
				withDockerConfig(
					withCredentials("url1", tokenAuth("kid1", ts(-5*time.Minute), ts(30*time.Minute))),
				),
			),
			wantRefreshNow: false,
			wantRefreshAt:  tsPtr((-5 + 21) * time.Minute),
		},
		{
			name: "token near expiry triggers refresh",
			urls: []string{"url1"},
			kids: []string{"kid1"},
			secret: secret(
				withDockerConfig(
					withCredentials("url1", tokenAuth("kid1", ts(-30*time.Minute), ts(5*time.Minute))),
				),
			),
			wantRefreshNow: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			refreshNow, refreshAt := isSecretRefreshNeeded(tc.secret, tc.urls, tc.kids, now)
			if refreshNow != tc.wantRefreshNow {
				t.Errorf("expected refreshNow=%v, got %v", tc.wantRefreshNow, refreshNow)
			}
			if refreshAt != tc.wantRefreshAt {
				if refreshAt == nil || tc.wantRefreshAt == nil || refreshAt.Compare(*tc.wantRefreshAt) != 0 {
					t.Errorf("expected refreshAt=%v, got %v", tc.wantRefreshAt, refreshAt)
				}
			}
		})
	}
}
