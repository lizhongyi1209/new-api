package constant

type TaskPlatform string

const (
	TaskPlatformSuno          TaskPlatform = "suno"
	TaskPlatformMidjourney                 = "mj"
	TaskPlatformAsyncImage                 = "async_image"
	TaskPlatformUnifiedImage               = "unified_image"
	TaskPlatformGenerateImage              = "generate_image"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate          = "generate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
	TaskActionMotionControl     = "motionControl"
	TaskActionMotionControl30   = "motionControl30"
	TaskActionOmniVideo         = "omniVideo"
	TaskActionOmniVideo30       = "omniVideo30"
	// NOTE: When adding a new video task action here, you MUST also register it
	// in the frontend "is this a video task?" allowlists, or the Task Logs page
	// will silently show "-" instead of a "preview video" link even when the
	// task's result_url is present. The Task Logs details column reads
	// result_url directly from the tasks table (see relay.TaskModel2Dto ->
	// task.GetResultURL()); it does NOT read logs.other.video_url. Update:
	//   - web/default: VIDEO_TASK_ACTIONS in
	//     src/features/usage-logs/components/columns/task-logs-columns.tsx
	//     plus TASK_ACTIONS / TASK_ACTION_MAPPINGS in
	//     src/features/usage-logs/constants.ts
	//   - web/classic: TASK_ACTION_* in src/constants/common.constant.js and the
	//     isVideoTask check in
	//     src/components/table/task-logs/TaskLogsColumnDefs.jsx
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
