package constant

import "testing"

func TestOfficialVideoPluginAllowed(t *testing.T) {
	tests := []struct {
		channelType int
		model, key  string
		want        bool
	}{
		{ChannelTypeKling, "kling-4.0", KlingOfficialVideoPlugin, true},
		{ChannelTypeKling, "kling-v3", KlingOfficialVideoPlugin, true},
		{ChannelTypeKling, "kling-4.0-flash", KlingVideoPlugin, true},
		{ChannelTypeKling, "kling-3.0", KlingVideoPlugin, true},
		{ChannelTypeKling, "kling-v3", KlingVideoPlugin, false},
		{ChannelTypeKling, KlingImage40Model, KlingVideoPlugin, true},
		{ChannelTypeKling, KlingImage40Model, KlingOfficialVideoPlugin, false},
		{ChannelTypeKling, "kling-4.0", WanOfficialVideoPlugin, false},
		{ChannelTypeKling, "kling-4.0", "", false},
		{ChannelTypeAli, "kling-4.0", KlingVideoPlugin, false},
		{ChannelTypeAli, "wan3.0-video", WanOfficialVideoPlugin, true},
	}
	for _, test := range tests {
		if got := OfficialVideoPluginAllowed(test.channelType, test.model, test.key); got != test.want {
			t.Errorf("OfficialVideoPluginAllowed(%d, %q, %q) = %v, want %v", test.channelType, test.model, test.key, got, test.want)
		}
	}
	if !IsOfficialVideoPlugin(KlingVideoPlugin) || !IsKlingOfficialVideoPlugin(KlingVideoPlugin) || IsKlingOfficialVideoPlugin(WanOfficialVideoPlugin) {
		t.Fatal("kling-video-plugin must be an official Kling video plugin")
	}
}
