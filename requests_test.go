package requests

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

//func Test_Download(t *testing.T) {
//	t.Log("Testing Download")
//	err := DownloadFile("https://github.com/prometheus/prometheus/releases/download/v2.12.0/prometheus-2.12.0.linux-amd64.tar.gz", true)
//	t.Log(err)
//}

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
		Hosts(map[string][]string{"127.0.0.1:8080": {"192.168.1.1:80"}, "4.org:80": {"httpbin.org:80"}}),
		//Proxy("http://127.0.0.1:8080"),
	)

	resp, err := sess.DoRequest(context.Background(), Method("GET"), URL("http://httpbin.org"), TraceLv(9))
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
		Logf(func(context.Context, Stat) {
			fmt.Println("session")

		}),
	)
	//if err := sess.Proxy("127.0.0.1:8080"); err != nil {
	//	t.Error(err)
	//	return
	//}

	resp, err := sess.DoRequest(context.Background(), Hosts(map[string][]string{"httpbin.org:80": {"aaa.com"}}),
		Method("POST"),
		URL("http://httpbin.org/post"),
		Params(map[string]any{
			"a": "b/c",
			"c": 3,
			"d": []int{1, 2, 3},
		}),
		Body(`{"body":"QWER"}`),
		Header("hello", "world"),
		TraceLv(9),
		//Logf(func(ctx context.Context, stat Stat) {
		//	fmt.Println(stat)
		//}),
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
		Params(map[string]any{
			"a": "b/c",
			"c": 3,
			"d": []int{1, 2, 3},
		}),
		TraceLv(9),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	t.Log(resp.StatusCode, err, resp.Response.ContentLength, resp.Request.ContentLength)

}

func Test_PostForm2(t *testing.T) {
	res, err := http.PostForm("http://httpbin.org/post", url.Values{
		"key":   {"this is url key"},
		"value": {"this is url value"},
	})
	// if err != nil {
	// 	t.Error(err)
	// 	return
	// }
	resp := Response{StartAt: time.Now(), Response: res, Err: err}
	t.Log("$$$$$$4", resp.Stat())
}

func Test_Race(t *testing.T) {
	opts := Options{}
	ctx := context.Background()
	t.Logf("%#v", opts)
	sess := New(URL("http://httpbin.org/post")) //, Auth("user", "123456"))
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = sess.DoRequest(ctx, MethodPost, Body(`{"a":"b"}`), Params(map[string]any{"1": "2/2"})) // nolint: errcheck
		}()
	}
	time.Sleep(3 * time.Second)
}

func Test_MockServer(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	defer s.Close()
	sess := New() //Hosts(map[string][]string{"qq.com:80": []string{"127.0.0.1"}}))
	resp, err := sess.DoRequest(context.Background(),
		URL("http://qq.com:80"), Path("/234"),
		//Body(map[string]string{"hello": "world"}),
		Body(strings.NewReader("12345678")),
		TraceLv(3, 102400),
		Logf(func(ctx context.Context, stat Stat) {
			t.Logf("%s\n", stat.String())
		}),
	)
	if err != nil {
		t.Error(err)
		return
	}
	t.Logf("%#v, %v", resp, err)
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
	body := `{}`
	s := New(URL("http://127.0.0.1:80/stream"), Body(body))
	resp, err := s.DoRequest(context.Background(), Method("POST"), Header("Content-Type", "application/json"), Stream(func(_ int64, b []byte) error {
		//fmt.Print(string(b))
		return nil
	}), TraceLv(3))
	t.Logf("%v, err=%v", resp.Stat(), err)

}

func Test_ForEach(t *testing.T) {
	s := New(RequestEach(func(ctx context.Context, req *http.Request) error {
		if req.Header.Get("RequestId") == "" {
			requestId, ok := ctx.Value("RequestId").(string)
			if !ok {
				requestId = "mytest"
			}
			req.Header.Set("Request-Id", requestId)
		}

		return nil
	}), ResponseEach(func(ctx context.Context, resp *http.Response) error {
		if resp.Header.Get("RequestId") == "" {
			requestId, ok := ctx.Value("RequestId").(string)
			if !ok {
				requestId = "myResponse"
			}
			resp.Header.Set("Request-Id", requestId)
		}

		return nil
	}))
	resp, err := s.DoRequest(context.Background(), URL("http://httpbin.org/get"), TraceLv(3, 10024))
	t.Logf("%v, %v", resp.Stat(), err)

}
