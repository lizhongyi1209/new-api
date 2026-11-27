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
		{ChannelTypeKling, "kling-4.0-flash", KlingOfficialPlugin, true},
		{ChannelTypeKling, "kling-3.0", KlingOfficialPlugin, true},
		{ChannelTypeKling, "kling-v3", KlingOfficialPlugin, false},
		{ChannelTypeKling, KlingImage40Model, KlingOfficialPlugin, true},
		{ChannelTypeKling, KlingImage40Model, KlingOfficialVideoPlugin, false},
		{ChannelTypeKling, "kling-4.0", WanOfficialVideoPlugin, false},
		{ChannelTypeKling, "kling-4.0", "", false},
		{ChannelTypeAli, "kling-4.0", KlingOfficialPlugin, false},
		{ChannelTypeAli, "wan3.0-video", WanOfficialVideoPlugin, true},
	}
	for _, test := range tests {
		if got := OfficialVideoPluginAllowed(test.channelType, test.model, test.key); got != test.want {
			t.Errorf("OfficialVideoPluginAllowed(%d, %q, %q) = %v, want %v", test.channelType, test.model, test.key, got, test.want)
		}
	}
	if !IsOfficialVideoPlugin(KlingOfficialPlugin) || !IsKlingOfficialVideoPlugin(KlingOfficialPlugin) || IsKlingOfficialVideoPlugin(WanOfficialVideoPlugin) {
		t.Fatal("kling-official-plugin must be an official Kling video plugin")
	}
}
