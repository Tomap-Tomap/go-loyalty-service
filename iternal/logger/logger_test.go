package logger

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_loggingResponseWriter_Write(t *testing.T) {
	type fields struct {
		ResponseWriter http.ResponseWriter
		bytes          int
	}
	type args struct {
		b []byte
	}
	type want struct {
		wantRet  int
		wantSize int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "test zero",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
			},
			args:    args{[]byte{}},
			want:    want{0, 0},
			wantErr: false,
		},
		{
			name: "test not zero",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
			},
			args:    args{[]byte{1, 2, 3}},
			want:    want{3, 3},
			wantErr: false,
		},
		{
			name: "test not empty resData",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
				bytes:          3,
			},
			args:    args{[]byte{1, 2, 3}},
			want:    want{3, 6},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &loggingResponseWriter{
				ResponseWriter: tt.fields.ResponseWriter,
				bytes:          tt.fields.bytes,
			}
			got, err := r.Write(tt.args.b)

			if tt.wantErr {
				require.Error(t, err)
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.wantRet, got)
			assert.Equal(t, tt.want.wantSize, r.bytes)
		})
	}
}

func Test_loggingResponseWriter_WriteHeader(t *testing.T) {
	type fields struct {
		ResponseWriter http.ResponseWriter
		code           int
	}
	type args struct {
		statusCode int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "test 200",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
			},
			args: args{200},
			want: 200,
		},
		{
			name: "test 400",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
				code:           123,
			},
			args: args{400},
			want: 400,
		},
		{
			name: "test not empty resData",
			fields: fields{
				ResponseWriter: httptest.NewRecorder(),
			},
			args: args{400},
			want: 400,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &loggingResponseWriter{
				ResponseWriter: tt.fields.ResponseWriter,
				code:           tt.fields.code,
			}
			r.WriteHeader(tt.args.statusCode)

			assert.Equal(t, tt.want, r.code)
		})
	}
}

func TestInitialize(t *testing.T) {
	type args struct {
		level string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "positive test",
			args:    args{"INFO"},
			wantErr: false,
		},
		{
			name:    "negative test",
			args:    args{"TEST"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.args.level, "stderr")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type testingSink struct {
	*bytes.Buffer
}

func (s *testingSink) Close() error { return nil }
func (s *testingSink) Sync() error  { return nil }

func TestRequestLogger(t *testing.T) {
	sink := &testingSink{new((bytes.Buffer))}
	zap.RegisterSink("testing", func(u *url.URL) (zap.Sink, error) { return sink, nil })
	Initialize("INFO", "testing://")

	type args struct {
		h http.Handler
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test",
			args: args{
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
			want: []string{`"level":"info"`, `"ts":`, `"caller":`, `"msg":"Sending HTTP response"`,
				`"duration":`, `"status":0`, `"size":0`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(RequestLogger(tt.args.h))
			req := resty.New().R()
			req.Method = http.MethodGet
			req.URL = srv.URL
			_, err := req.Send()
			assert.NoError(t, err)

			logs := sink.String()

			for _, val := range tt.want {
				assert.Contains(t, logs, val)
			}

			srv.Close()
		})
	}
}
