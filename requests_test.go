package requests

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func Test_Basic(t *testing.T) {
	resp, err := Get("http://127.0.0.1:12345/get")
	t.Logf("%#v, %v", resp, err)
	//resp, _ = Post("http://httpbin.org/post", "application/json", strings.NewReader(`{"a": "b"}`))
	//t.Log(resp.Text())
}

func Test_ProxyGet(t *testing.T) {
	t.Log("Testing get request")
	sess := New(
		Header("a", "b"),
		Cookie(http.Cookie{Name: "username", Value: "golang"}),
		BasicAuth("user", "123456"),
		Timeout(3*time.Second),
		//Hosts(map[string][]string{"127.0.0.1:8080": {"192.168.1.1:80"}, "4.org:80": {"httpbin.org:80"}}),
		//Proxy("http://127.0.0.1:8080"),
	)

	resp, err := sess.DoRequest(
		context.Background(),
		Method("GET"),
		URL("http://httpbin.org"),
	)
	if err != nil {
		t.Errorf("%s", err.Error())
		return
	}
	t.Log(resp.StatusCode, err)
	//t.Log(resp.Text())
}

func Test_PostBody(t *testing.T) {
	sess := New(
		BasicAuth("user", "123456"),
		Logf(func(context.Context, *Stat) {
			fmt.Println("session")

		}),
	)
	//if err := sess.Proxy("127.0.0.1:8080"); err != nil {
	//	t.Error(err)
	//	return
	//}

	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL("http://httpbin.org/post"),
		Params(map[string]string{
			"a": "b/c",
			"c": "3",
			"d": "ddd",
		}),
		Param("e", "ea", "es"),

		Body(`{"body":"QWER"}`),
		Header("hello", "world"),
		//TraceLv(9),
		Logf(func(ctx context.Context, stat *Stat) {
			t.Logf("%v", stat.String())
		}),
	)
	if err != nil {
		t.Logf("%v", err)
		return
	}
	t.Log(resp.StatusCode, err, resp.Response.ContentLength, resp.Request.ContentLength)
	//t.Log(resp.Text())
	//t.Log(resp.Stat())
}

func Test_FormPost(t *testing.T) {
	t.Log("Testing get request")

	sess := New()
	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL("http://httpbin.org/post"),
		Form(url.Values{"name": {"12.com"}}),
		Params(map[string]string{
			"a": "b/c",
			"c": "cc",
			"d": "dddd",
		}),
		Param("e", "ea", "es"),

		//TraceLv(9),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	t.Log(resp.StatusCode, err, resp.Response.ContentLength, resp.Request.ContentLength)

}

func Test_Race(t *testing.T) {
	opts := Options{}
	ctx := context.Background()
	t.Logf("%#v", opts)
	sess := New(URL("http://httpbin.org/post")) //, Auth("user", "123456"))
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = sess.DoRequest(ctx, MethodPost, Body(`{"a":"b"}`), Params(map[string]string{"1": "2/2"})) // nolint: errcheck
		}()
	}
	time.Sleep(3 * time.Second)
}

func Test_Retry(t *testing.T) {
	var reqCount int32 = 0

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNo := atomic.AddInt32(&reqCount, 1)
		if reqNo%3 == 0 {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(200)
		}
		_, _ = w.Write([]byte(fmt.Sprintf("response: %d", reqNo)))
	}))
	defer s.Close()

	sess := New()
	_, _ = sess.DoRequest(context.Background(), URL(s.URL))
}

func Test_Cannel(t *testing.T) {
	sess := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := sess.DoRequest(ctx, URL("http://127.0.0.1:9099"))
	t.Logf("%s, err=%v", resp.Stat(), err)
}

func Test_Stream(t *testing.T) {
	body := `{"Namespace":"v_mix_vm", "ResultColumn":["UUID", "AccountName"], "Limit": 100}`
	s := New(URL("http://cache-api.polaris:80/stream"), Body(body))
	resp, err := s.DoRequest(context.Background(), MethodPost,
		Header("Content-Type", "application/json"),
		//TraceLv(3),
		Logf(func(ctx context.Context, stat *Stat) {
			fmt.Println(stat)
		}),
		Stream(func(_ int64, b []byte) error {
			fmt.Print(string(b))
			return nil
		}))
	t.Logf("%v, err=%v", resp.Stat(), err)

}

func TestResponse_Download(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		text := "abc\ndef\nghij\n\n123"
		fmt.Fprint(w, text)
	}))
	defer s.Close()

	u := "https://go.dev/dl/go1.22.1.darwin-amd64.tar.gz" // a35015fca6f631f3501a36b3bccba9c5
	sess := New(URL(u))
	f, err := os.OpenFile("tmp/xx.tar.gz", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	defer f.Close()
	sum := 0
	resp, err := sess.DoRequest(context.Background(),
		Stream(func(i int64, row []byte) error {
			cnt, err := f.Write(row)
			sum += cnt
			return err
		}))
	if err != nil {
		t.Logf("resp=%d, err=%s", resp.Content, err)
		return
	}
	t.Logf("resp=%d, err=%s", resp.Content, err)
}
