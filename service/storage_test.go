package service

import "testing"

func TestSelectImageStorageProviderDefaultHosts(t *testing.T) {
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "")
	t.Setenv("R2_STORAGE_HOSTS", "")

	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "api host", host: "api.o1key.cn", want: ImageStorageProviderAliyunOSS},
		{name: "api host with port", host: "api.o1key.cn:443", want: ImageStorageProviderAliyunOSS},
		{name: "api URL", host: "https://api.o1key.cn/v1/images/generations", want: ImageStorageProviderAliyunOSS},
		{name: "cf api cn host", host: "cf-api.o1key.cn", want: ImageStorageProviderR2},
		{name: "cf api com host", host: "cf-api.o1key.com", want: ImageStorageProviderR2},
		{name: "unknown host defaults to r2", host: "example.com", want: ImageStorageProviderR2},
		{name: "empty host defaults to r2", host: "", want: ImageStorageProviderR2},
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
	t.Setenv("ALIYUN_OSS_STORAGE_HOSTS", "api.example.com, https://img-api.example.com")
	t.Setenv("R2_STORAGE_HOSTS", "cf.example.com")

	if got := SelectImageStorageProvider("img-api.example.com:8443"); got != ImageStorageProviderAliyunOSS {
		t.Fatalf("custom OSS host selected %q, want %q", got, ImageStorageProviderAliyunOSS)
	}
	if got := SelectImageStorageProvider("cf.example.com"); got != ImageStorageProviderR2 {
		t.Fatalf("custom R2 host selected %q, want %q", got, ImageStorageProviderR2)
	}
	if got := SelectImageStorageProvider("api.o1key.cn"); got != ImageStorageProviderR2 {
		t.Fatalf("default OSS host should not match when custom env is set; got %q", got)
	}
}
