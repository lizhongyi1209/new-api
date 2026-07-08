package service

import "testing"

func TestSelectImageStorageProviderDefaultHosts(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "")
	t.Setenv("R2_STORAGE_HOSTS", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "")

	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "api host uses local storage", host: "api.o1key.cn", want: ImageStorageProviderLocal},
		{name: "api host with port", host: "api.o1key.cn:443", want: ImageStorageProviderLocal},
		{name: "api URL", host: "https://api.o1key.cn/v1/images/generations", want: ImageStorageProviderLocal},
		{name: "cf api cn host", host: "cf-api.o1key.cn", want: ImageStorageProviderR2},
		{name: "cf api com host", host: "cf-api.o1key.com", want: ImageStorageProviderR2},
		{name: "unknown host defaults to local", host: "example.com", want: ImageStorageProviderLocal},
		{name: "empty host defaults to local", host: "", want: ImageStorageProviderLocal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SelectImageStorageProvider(test.host); got != test.want {
				t.Fatalf("SelectImageStorageProvider(%q) = %q, want %q", test.host, got, test.want)
			}
		})
	}
}

func TestSelectImageStorageProviderCustomHosts(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "img-local.example.com")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "api.example.com, https://img-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "cf.example.com")
	t.Setenv("DISABLE_ALIYUN_OSS", "")

	if got := SelectImageStorageProvider("img-local.example.com"); got != ImageStorageProviderLocal {
		t.Fatalf("custom local host selected %q, want %q", got, ImageStorageProviderLocal)
	}
	if got := SelectImageStorageProvider("img-api.example.com:8443"); got != ImageStorageProviderAliyunOSS {
		t.Fatalf("custom OSS host selected %q, want %q", got, ImageStorageProviderAliyunOSS)
	}
	if got := SelectImageStorageProvider("cf.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("custom R2 host selected %q, want %q", got, ImageStorageProviderR2)
	}
}

// DISABLE_ALIYUN_OSS 只把 OSS 分支改写为 R2，不得影响本地存储路由（2e7292635 回归）。
func TestSelectImageStorageProviderOSSKillSwitch(t *testing.T) {
	t.Setenv("LOCAL_STORAGE_HOSTS", "")
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "oss-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "")
	t.Setenv("DISABLE_ALIYUN_OSS", "true")

	if got := SelectImageStorageProvider("oss-api.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("kill-switch should redirect OSS host to R2, got %q", got)
	}
	if got := SelectImageStorageProvider("api.o1key.cn"); got != ImageStorageProviderLocal {
		t.Fatalf("kill-switch must not affect local storage routing, got %q", got)
	}
}
