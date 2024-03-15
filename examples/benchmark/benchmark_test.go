package benchmark

import (
	"context"
	"github.com/golang-io/requests"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// go test -v -bench='Benchmark_requests' -benchmem .
func Benchmark_requests(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		_, _ = io.Copy(w, r.Body)
	}))
	defer s.Close()
	sess := requests.New(requests.URL(s.URL), requests.Proxy("http://128.0.0.1:80"))
	b.ResetTimer()

	for i := 0; i <= b.N; i++ {
		_, err := sess.DoRequest(context.Background(),
			requests.Path("/234"),
			//Body(map[string]string{"hello": "world"}),
			requests.Body(strings.NewReader("12345678")),
			//TraceLv(3, 102400),
			requests.Logf(func(ctx context.Context, stat requests.Stat) {
				b.Logf("%s\n", stat.String())
			}),
		)
		if err != nil {
			b.Error(err)
			return
		}
		//b.Logf("%#v, %v", resp, err)
	}

}

func Test_requests(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second)
		_, _ = io.Copy(w, r.Body)
	}))
	defer s.Close()
	sess := requests.New(requests.URL(s.URL), requests.Timeout(3*time.Second))

	_, err := sess.DoRequest(context.Background(), requests.Path("/234"), requests.Body(strings.NewReader("12345678")),
		//requests.TraceLv(3, 102400),

		requests.Logf(func(ctx context.Context, stat requests.Stat) {
			t.Logf("%s\n", stat.String())
		}),
	)
	if err != nil {
		t.Error(err)
		return
	}
	//b.Logf("%#v, %v", resp, err)

}

// go test -v -bench='Benchmark_requests' -benchmem .
func Test_Proxy(t *testing.T) {
	sess := requests.New(requests.Proxy("http://127.0.0.1:60001"))
	resp, err := sess.DoRequest(context.Background(), requests.MethodPost, requests.URL("http://httpbin.org/post"),
		requests.Body(strings.NewReader("12345678")),
		requests.Logf(func(ctx context.Context, stat requests.Stat) {
			t.Logf("%s\n", stat.String())
		}),
		//requests.TraceLv(3),
	)
	if err != nil {
		t.Error(err)
		return
	}

	t.Logf("%#v, %v", resp.Text(), err)
}

// go test -v -bench='Benchmark_requests' -benchmem .
func Test_NoProxy(t *testing.T) {
	sess := requests.New()
	resp, err := sess.DoRequest(context.Background(), requests.MethodGet, requests.URL("http://httpbin.org/get"),
		requests.Body(strings.NewReader("12345678")),
		requests.Logf(func(ctx context.Context, stat requests.Stat) {
			t.Logf("%s\n", stat.String())
		}),
		//requests.TraceLv(3),
	)
	if err != nil {
		t.Error(err)
		return
	}
	t.Logf("%#v, %v", resp.Text(), err)
}
