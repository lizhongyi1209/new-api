package system_setting

var ServerAddress = "http://localhost:3000"

// TaskPublicAddress is used only when building public plugin task artifact links.
// An empty value keeps ServerAddress as the fallback.
var TaskPublicAddress = ""
var WorkerUrl = ""
var WorkerValidKey = ""
var WorkerAllowHttpImageRequestEnabled = false

func EnableWorker() bool {
	return WorkerUrl != ""
}
