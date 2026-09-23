import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowRight,
  ClipboardPaste,
  HelpCircle,
  Loader2,
  Sparkles,
  Trash2,
  Copy,
  FileText,
  Eraser,
  Plus,
  Eye,
  RefreshCw,
  Code,
  Route,
  Settings,
  SlidersHorizontal,
  Wand2,
} from 'lucide-react'
import {
  type ReactNode,
  useEffect,
  useState,
  useMemo,
  useCallback,
  useRef,
} from 'react'
import { type SubmitErrorHandler, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSectionClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { ErrorState } from '@/components/error-state'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { JsonEditor } from '@/components/json-editor'
import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SecureVerificationDialog } from '@/features/auth/secure-verification'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useHiddenClickUnlock } from '@/hooks/use-hidden-click-unlock'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import {
  parseChannelConnectionInfo,
  type ChannelConnectionInfo,
} from '@/lib/channel-connection-info'
import { ROLE } from '@/lib/roles'
import {
  requireServerSuccess,
  getServerErrorMessage,
} from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  getAllModels,
  getChannel,
  getChannelDefaultBaseURLs,
  getGroups,
  getPrefillGroups,
  getTaskPluginOptions,
  refreshCodexCredential,
} from '../../api'
import {
  ADD_MODE_OPTIONS,
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPE_TASK_PLUGIN,
  CHANNEL_TYPE_WARNINGS,
  CLAUDE_FIELD_PASSTHROUGH_TYPES,
  ERROR_MESSAGES,
  FIELD_DESCRIPTIONS,
  FIELD_PASSTHROUGH_TYPES,
  FIELD_PLACEHOLDERS,
  GEMINI_FILE_DATA_CHANNEL_TYPES,
  MODEL_FETCHABLE_TYPES,
  OPENAI_FIELD_PASSTHROUGH_TYPES,
} from '../../constants'
import { useChannelKeyDisclosure } from '../../hooks/use-channel-key-disclosure'
import { useChannelMutateForm } from '../../hooks/use-channel-mutate-form'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  CHANNEL_TYPE_ADVANCED_CUSTOM,
  channelFormSchema,
  channelsQueryKeys,
  getAdvancedCustomStats,
  transformChannelToFormDefaults,
  type ChannelFormValues,
  deduplicateKeys,
  getKeyPromptForType,
  resolveSeedanceMaxChannelModelDefaults,
  parseModelsString,
  formatModelsArray,
  extractRedirectModels,
  extractMappingSourceModels,
  hasModelConfigChanged,
  findMissingModelsInMapping,
  validateModelMappingJson,
} from '../../lib'
import {
  getChannelConfigurationSectionForField,
  type ChannelConfigurationSection,
  type ChannelConfigurationStatus,
} from '../../lib/channel-configuration'
import {
  collectInvalidStatusCodeEntries,
  collectNewDisallowedStatusCodeRedirects,
} from '../../lib/status-code-risk-guard'
import type { Channel } from '../../types'
import { ChannelModelDiscovery } from '../channel-model-discovery'
import { ChannelTypeLogo } from '../channel-type-logo'
import { useChannels } from '../channels-provider'
import { AdvancedCustomEditorDialog } from '../dialogs/advanced-custom-editor-dialog'
import {
  MissingModelsConfirmationDialog,
  type MissingModelsAction,
} from '../dialogs/missing-models-confirmation-dialog'
import { ParamOverrideEditorDialog } from '../dialogs/param-override-editor-dialog'
import { StatusCodeRiskDialog } from '../dialogs/status-code-risk-dialog'
import { ModelMappingEditor } from '../model-mapping-editor'
import { ChannelConfiguration } from './channel-configuration'
import { ChannelProviderPicker } from './channel-provider-picker'
import {
  ChannelApiAccessSection,
  ChannelAuthSection,
  ChannelBasicSection,
  ChannelEditorLoadingState,
  ChannelModelsSection,
} from './sections'

type ChannelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Channel | null
}

type ModelMappingGuardrail = {
  invalidJson: boolean
  entries: Array<{ source: string; target: string }>
  missingSourceModels: string[]
  exposedTargetModels: string[]
}

// Helper functions
const createEmptyModelMappingGuardrail = (): ModelMappingGuardrail => ({
  invalidJson: false,
  entries: [],
  missingSourceModels: [],
  exposedTargetModels: [],
})

const formatModelNames = (models: string[]): string =>
  models.map((model) => `"${model}"`).join(', ')

const MODEL_MAPPING_PREVIEW_FALLBACK: Array<{
  source: string
  target: string
}> = [{ source: 'client-model', target: 'upstream-model' }]

const CHANNEL_EDITOR_SECTION_IDS = {
  identity: 'channel-section-identity',
  credentials: 'channel-section-credentials',
  models: 'channel-section-models',
  advanced: 'channel-section-advanced',
} as const
const ADVANCED_SETTINGS_SECTION_IDS = {
  routingStrategy: 'channel-section-advanced-routing-strategy',
  internalNotes: 'channel-section-advanced-internal-notes',
  overrideRules: 'channel-section-advanced-override-rules',
  extraSettings: 'channel-section-advanced-extra-settings',
  imageOutput: 'channel-section-advanced-image-output',
  fieldPassthrough: 'channel-section-advanced-field-passthrough',
  upstreamModelDetection: 'channel-section-advanced-upstream-model-detection',
} as const
const ADVANCED_CUSTOM_ROUTE_TYPE_PREVIEW_LIMIT = 3
const UPSTREAM_DETECTED_MODEL_PREVIEW_LIMIT = 8
const SENSITIVE_FORM_FIELDS = [
  'type',
  'base_url',
  'key',
  'openai_organization',
  'other',
  'key_mode',
  'param_override',
  'header_override',
  'settings',
  'setting',
  'advanced_custom',
  'is_enterprise_account',
  'vertex_key_type',
  'aws_key_type',
  'azure_responses_version',
  'image_output_strategy',
  'gemini_file_data_enabled',
  'force_format',
  'thinking_to_content',
  'proxy',
  'http_protocol',
  'http2_connection_shards',
  'pass_through_body_enabled',
  'system_prompt',
  'system_prompt_override',
  'allow_service_tier',
  'disable_store',
  'allow_safety_identifier',
  'allow_include_obfuscation',
  'allow_inference_geo',
  'allow_speed',
  'claude_beta_query',
  'disable_task_polling_sleep',
  'upstream_model_update_check_enabled',
  'upstream_model_update_auto_sync_enabled',
  'upstream_model_update_ignored_models',
] satisfies (keyof ChannelFormValues)[]

function hasConfiguredOverrideValue(value: unknown): boolean {
  if (typeof value !== 'string') return false

  const trimmed = value.trim()
  if (!trimmed || trimmed === 'null') return false

  try {
    const parsed = JSON.parse(trimmed)
    if (parsed === null) return false
    if (Array.isArray(parsed)) return parsed.length > 0
    if (typeof parsed === 'object') return Object.keys(parsed).length > 0
  } catch {
    return true
  }

  return true
}

function parseSettingsRecord(
  settings: string | undefined
): Record<string, unknown> {
  if (!settings?.trim()) return {}
  try {
    const parsed = JSON.parse(settings)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function formatUnixTime(timestamp: unknown): string {
  const seconds = Number(timestamp)
  if (!Number.isFinite(seconds) || seconds <= 0) return '-'
  return new Date(seconds * 1000).toLocaleString()
}

function CardHeading(props: {
  title: string
  icon?: ReactNode
  iconTone?: IconBadgeTone
}) {
  return (
    <div className='flex items-center gap-3'>
      {props.icon && (
        <IconBadge tone={props.iconTone} size='md'>
          {props.icon}
        </IconBadge>
      )}
      <h3 className='text-sm font-semibold tracking-tight'>{props.title}</h3>
    </div>
  )
}

function SubHeading(props: {
  title: string
  icon?: ReactNode
  iconTone?: IconBadgeTone
}) {
  return (
    <div className='flex items-center gap-2'>
      {props.icon && (
        <IconBadge tone={props.iconTone} size='xs'>
          {props.icon}
        </IconBadge>
      )}
      <h4 className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
        {props.title}
      </h4>
    </div>
  )
}

function configuredAdvancedSectionClassName(
  className: string,
  configured: boolean
) {
  return cn(
    className,
    'border-border/60 rounded-lg border p-3 transition-colors',
    configured && 'border-primary/35 ring-primary/20 ring-1'
  )
}

export function ChannelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ChannelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { setOpen } = useChannels()
  const currentUser = useAuthStore((s) => s.auth.user)
  const canEditSensitive = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
  )
  const canRevealChannelKey = currentUser?.role === ROLE.SUPER_ADMIN
  const canOperateChannel = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.OPERATE
  )
  const canBindTaskPlugin = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.TASK_PLUGIN,
    ADMIN_PERMISSION_ACTIONS.BIND
  )
  const [isCodexCredentialRefreshing, setIsCodexCredentialRefreshing] =
    useState(false)
  const initialModelsRef = useRef<string[]>([])
  const initialModelMappingRef = useRef<string>('')
  const initialStatusCodeMappingRef = useRef<string>('')
  const [statusCodeRiskOpen, setStatusCodeRiskOpen] = useState(false)
  const [statusCodeRiskDetailItems, setStatusCodeRiskDetailItems] = useState<
    string[]
  >([])
  const statusCodeRiskResolveRef = useRef<
    ((confirmed: boolean) => void) | null
  >(null)
  const [missingModelsDialogOpen, setMissingModelsDialogOpen] = useState(false)
  const [missingModelsList, setMissingModelsList] = useState<string[]>([])
  const missingModelsResolveRef = useRef<
    ((action: MissingModelsAction) => void) | null
  >(null)
  const [providerPickerOpen, setProviderPickerOpen] = useState(!currentRow)
  const [providerChosen, setProviderChosen] = useState(Boolean(currentRow))
  const channelFormRef = useRef<HTMLFormElement>(null)
  const [configurationSection, setConfigurationSection] =
    useState<ChannelConfigurationSection>('connection')
  const [focusField, setFocusField] = useState<keyof ChannelFormValues | null>(
    null
  )
  const [paramOverrideEditorOpen, setParamOverrideEditorOpen] = useState(false)
  const [advancedCustomEditorOpen, setAdvancedCustomEditorOpen] =
    useState(false)
  const [clipboardConnectionInfo, setClipboardConnectionInfo] =
    useState<ChannelConnectionInfo | null>(null)

  const isEditing = Boolean(currentRow)
  const channelId = currentRow?.id ?? null
  const sensitiveLocked = isEditing && !canEditSensitive
  useEffect(() => {
    if (open) {
      setProviderPickerOpen(!isEditing)
      setProviderChosen(isEditing)
    }
  }, [open, isEditing, channelId])

  // Fetch channel details if editing
  const {
    data: channelData,
    isLoading: isChannelLoading,
    isError: isChannelError,
    error: channelError,
    refetch: refetchChannel,
  } = useQuery({
    queryKey: channelsQueryKeys.detail(channelId || 0),
    queryFn: async () => requireServerSuccess(await getChannel(channelId || 0)),
    enabled: open && isEditing && Boolean(channelId),
    meta: { errorToast: false, errorRedirect: false },
  })

  // Fetch available groups
  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })

  // Fetch all available models
  const { data: defaultBaseURLs } = useQuery({
    queryKey: ['channel_default_base_urls'],
    meta: { errorToast: false, errorRedirect: false },
    queryFn: getChannelDefaultBaseURLs,
    enabled: open,
    staleTime: 10 * 60 * 1000,
    retry: false,
  })

  const taskPluginOptionsQuery = useQuery({
    queryKey: ['task-plugin-options'],
    queryFn: getTaskPluginOptions,
    enabled: open && canBindTaskPlugin,
    meta: { errorToast: false, errorRedirect: false },
    retry: false,
  })

  const { data: allModelsData } = useQuery({
    queryKey: ['channel_models'],
    queryFn: async () => requireServerSuccess(await getAllModels()),
  })

  // Fetch prefill model groups
  const { data: prefillGroupsData } = useQuery({
    queryKey: ['prefill_groups', 'model'],
    queryFn: async () => requireServerSuccess(await getPrefillGroups('model')),
  })

  const { copyToClipboard } = useCopyToClipboard()

  const { channelKey, isChannelKeyLoading, handleRevealKey, verification } =
    useChannelKeyDisclosure(open, channelId)

  // Check if this is a multi-key channel
  const isMultiKeyChannel =
    isEditing && channelData?.data?.channel_info?.is_multi_key === true

  // Form setup
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: CHANNEL_FORM_DEFAULT_VALUES,
    shouldFocusError: false,
  })

  // Watch form values for conditional rendering
  const multiKeyMode = form.watch('multi_key_mode')
  const multiKeyType = form.watch('multi_key_type')
  const keyMode = form.watch('key_mode')
  const currentGroups = form.watch('group')
  const currentType = form.watch('type')
  const currentBaseUrl = form.watch('base_url')
  const currentKey = form.watch('key')
  const currentOther = form.watch('other')
  const currentModels = form.watch('models')
  const currentName = form.watch('name')
  const currentModelMapping = form.watch('model_mapping')
  const awsKeyType = form.watch('aws_key_type')
  const vertexKeyType = form.watch('vertex_key_type')
  const upstreamModelUpdateCheckEnabled = form.watch(
    'upstream_model_update_check_enabled'
  )
  const currentSettings = form.watch('settings')
  const currentAdvancedCustom = form.watch('advanced_custom')
  const currentPriority = form.watch('priority')
  const currentWeight = form.watch('weight')
  const currentTestModel = form.watch('test_model')
  const currentAutoBan = form.watch('auto_ban')
  const currentTag = form.watch('tag')
  const currentRemark = form.watch('remark')
  const currentStatusCodeMapping = form.watch('status_code_mapping')
  const currentParamOverride = form.watch('param_override')
  const currentHeaderOverride = form.watch('header_override')
  const currentForceFormat = form.watch('force_format')
  const currentThinkingToContent = form.watch('thinking_to_content')
  const currentPassThroughBodyEnabled = form.watch('pass_through_body_enabled')
  const currentDisableTaskPollingSleep = form.watch(
    'disable_task_polling_sleep'
  )
  const currentGeminiFileDataEnabled = form.watch('gemini_file_data_enabled')
  const currentImageOutputStrategy = form.watch('image_output_strategy')
  const currentProxy = form.watch('proxy')
  const currentHttpProtocol = form.watch('http_protocol')
  const currentHttp2ConnectionShards = form.watch('http2_connection_shards')
  const currentSystemPrompt = form.watch('system_prompt')
  const currentSystemPromptOverride = form.watch('system_prompt_override')
  const currentAllowServiceTier = form.watch('allow_service_tier')
  const currentDisableStore = form.watch('disable_store')
  const currentAllowSafetyIdentifier = form.watch('allow_safety_identifier')
  const currentAllowIncludeObfuscation = form.watch('allow_include_obfuscation')
  const currentAllowInferenceGeo = form.watch('allow_inference_geo')
  const currentAllowSpeed = form.watch('allow_speed')
  const currentClaudeBetaQuery = form.watch('claude_beta_query')
  const currentUpstreamModelUpdateAutoSyncEnabled = form.watch(
    'upstream_model_update_auto_sync_enabled'
  )
  const currentUpstreamModelUpdateIgnoredModels = form.watch(
    'upstream_model_update_ignored_models'
  )
  const {
    unlocked: doubaoApiEditUnlocked,
    handleClick: handleApiConfigSecretClick,
    reset: resetDoubaoApiUnlock,
  } = useHiddenClickUnlock({
    requiredClicks: 10,
    disabled: currentType !== 45 || sensitiveLocked,
    onUnlock: () => {
      toast.info(t('Doubao custom API address editing unlocked'))
    },
  })

  useEffect(() => {
    if (!open) {
      resetDoubaoApiUnlock()
    }
  }, [open, resetDoubaoApiUnlock])

  const applyConnectionInfo = useCallback(
    (connectionInfo: ChannelConnectionInfo) => {
      form.setValue('key', connectionInfo.key, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue('base_url', connectionInfo.url, {
        shouldDirty: true,
        shouldValidate: true,
      })
      setProviderChosen(true)
      setProviderPickerOpen(false)
      setConfigurationSection('connection')
      setClipboardConnectionInfo(null)
      toast.success(t('Connection info filled in'))
    },
    [form, t]
  )

  const pasteConnectionInfoFromClipboard = useCallback(async () => {
    if (typeof navigator === 'undefined' || !navigator.clipboard?.readText) {
      toast.error(t('Unable to read clipboard'))
      return
    }

    try {
      const text = await navigator.clipboard.readText()
      const parsed = parseChannelConnectionInfo(text)
      if (parsed) {
        applyConnectionInfo(parsed)
        return
      }
      toast.info(t('No connection info found in clipboard'))
    } catch {
      toast.error(t('Unable to read clipboard'))
    }
  }, [applyConnectionInfo, t])

  useEffect(() => {
    if (!open || isEditing) {
      setClipboardConnectionInfo(null)
      return
    }

    if (typeof navigator === 'undefined' || !navigator.clipboard?.readText) {
      return
    }

    let cancelled = false
    void navigator.clipboard
      .readText()
      .then((text) => {
        if (cancelled) return
        setClipboardConnectionInfo(parseChannelConnectionInfo(text))
      })
      .catch(() => {
        /* Clipboard detection is best-effort on drawer open. */
      })

    return () => {
      cancelled = true
    }
  }, [isEditing, open])

  // Helper computed values
  const isBatchMode =
    multiKeyMode === 'batch' || multiKeyMode === 'multi_to_single'
  const isChannelDetailLoading = isEditing && isChannelLoading
  const isChannelDetailUnavailable = isEditing && !channelData?.data
  const supportsMultiKeyAddMode =
    currentType !== 57 && !(currentType === 41 && vertexKeyType === 'api_key')
  const addModeOptions = useMemo(
    () =>
      supportsMultiKeyAddMode
        ? ADD_MODE_OPTIONS
        : ADD_MODE_OPTIONS.filter((option) => option.value === 'single'),
    [supportsMultiKeyAddMode]
  )

  const advancedCustomStats = useMemo(
    () => getAdvancedCustomStats(currentAdvancedCustom),
    [currentAdvancedCustom]
  )
  const advancedCustomRouteTypeLabels =
    advancedCustomStats.routeTypeLabels.slice(
      0,
      ADVANCED_CUSTOM_ROUTE_TYPE_PREVIEW_LIMIT
    )
  const hiddenAdvancedCustomRouteTypeCount =
    advancedCustomStats.routeTypeLabels.length -
    advancedCustomRouteTypeLabels.length
  const advancedCustomRouteTypeTitle =
    hiddenAdvancedCustomRouteTypeCount > 0
      ? advancedCustomStats.routeTypeLabels.join(', ')
      : undefined

  // Get all models list
  const allModelsList = useMemo(
    () => allModelsData?.data?.map((model) => model.id).filter(Boolean) || [],
    [allModelsData]
  )

  // Get basic models for the current channel type
  const basicModels = useMemo(() => {
    if (!allModelsList.length) return []
    // Filter models based on common patterns for specific types
    if (currentType === 1) {
      return allModelsList.filter(
        (model) => model.startsWith('gpt-') || model.startsWith('text-')
      )
    }
    return allModelsList
  }, [allModelsList, currentType])

  // Get prefill groups
  const prefillGroups = useMemo(
    () => prefillGroupsData?.data || [],
    [prefillGroupsData]
  )

  // Transform groups to multi-select options
  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    const allGroups = new Set([...groupsData.data, ...(currentGroups || [])])
    return [...allGroups].map((group) => ({
      value: group,
      label: group,
    }))
  }, [groupsData, currentGroups])

  // Parse current models as array
  const currentModelsArray = useMemo(
    () => parseModelsString(currentModels),
    [currentModels]
  )

  const currentTypeLabel = useMemo(
    () =>
      CHANNEL_TYPE_OPTIONS.find((option) => option.value === currentType)
        ?.label || `#${currentType}`,
    [currentType]
  )

  const formErrors = form.formState.errors
  const providerRequiresBaseUrl = [3, 8, 36, 45].includes(currentType)
  const providerRequiresOther = [3, 18, 21, 39, 41, 49].includes(currentType)
  const identityComplete = Boolean(currentName?.trim() && currentType > 0)
  const credentialsComplete = Boolean(
    (isEditing || currentKey?.trim()) &&
    (!providerRequiresBaseUrl || currentBaseUrl?.trim()) &&
    (!providerRequiresOther || currentOther?.trim())
  )
  const modelsComplete = Boolean(
    currentModelsArray.length > 0 && currentGroups?.length
  )
  const routingStrategyConfigured = Boolean(
    currentPriority ||
    currentWeight ||
    currentTestModel?.trim() ||
    (currentAutoBan ?? 1) !== 1
  )
  const internalNotesConfigured = Boolean(
    currentTag?.trim() || currentRemark?.trim()
  )
  const overrideRulesConfigured = Boolean(
    hasConfiguredOverrideValue(currentStatusCodeMapping) ||
    hasConfiguredOverrideValue(currentParamOverride) ||
    hasConfiguredOverrideValue(currentHeaderOverride)
  )
  const extraSettingsConfigured = Boolean(
    currentForceFormat ||
    currentThinkingToContent ||
    currentPassThroughBodyEnabled ||
    currentDisableTaskPollingSleep ||
    currentProxy?.trim() ||
    currentSystemPrompt?.trim() ||
    currentSystemPromptOverride ||
    (currentHttpProtocol && currentHttpProtocol !== 'auto') ||
    (currentHttp2ConnectionShards != null && currentHttp2ConnectionShards > 1)
  )
  const imageOutputConfigured = Boolean(
    (currentImageOutputStrategy &&
      currentImageOutputStrategy !== 'passthrough') ||
    currentGeminiFileDataEnabled
  )
  let fieldPassthroughConfigured = false
  if (OPENAI_FIELD_PASSTHROUGH_TYPES.has(currentType)) {
    fieldPassthroughConfigured = Boolean(
      currentAllowServiceTier ||
      currentDisableStore ||
      currentAllowSafetyIdentifier ||
      currentAllowIncludeObfuscation ||
      currentAllowInferenceGeo
    )
  }
  if (CLAUDE_FIELD_PASSTHROUGH_TYPES.has(currentType)) {
    fieldPassthroughConfigured =
      fieldPassthroughConfigured ||
      Boolean(
        currentAllowServiceTier ||
        currentAllowInferenceGeo ||
        currentAllowSpeed ||
        (currentType === 14 && currentClaudeBetaQuery)
      )
  }
  const upstreamModelDetectionConfigured = Boolean(
    upstreamModelUpdateCheckEnabled ||
    currentUpstreamModelUpdateAutoSyncEnabled ||
    currentUpstreamModelUpdateIgnoredModels?.trim()
  )
  const configurationStatuses: Record<
    ChannelConfigurationSection,
    ChannelConfigurationStatus
  > = {
    connection:
      identityComplete && credentialsComplete && modelsComplete
        ? 'ready'
        : 'idle',
    routing:
      routingStrategyConfigured ||
      hasConfiguredOverrideValue(currentModelMapping)
        ? 'configured'
        : 'idle',
    request:
      overrideRulesConfigured || fieldPassthroughConfigured
        ? 'configured'
        : 'idle',
    other:
      internalNotesConfigured ||
      extraSettingsConfigured ||
      imageOutputConfigured ||
      upstreamModelDetectionConfigured
        ? 'configured'
        : 'idle',
  }
  for (const field of Object.keys(formErrors)) {
    configurationStatuses[getChannelConfigurationSectionForField(field)] =
      'error'
  }

  // Extract redirect models from model_mapping (target values)
  const redirectModelList = useMemo(
    () => extractRedirectModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Extract source keys from model_mapping (models being remapped FROM)
  const redirectModelKeyList = useMemo(
    () => extractMappingSourceModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Transform models to multi-select options
  const modelOptions = useMemo(() => {
    const allModels = new Set([...allModelsList, ...currentModelsArray])
    return [...allModels].map((model) => ({
      value: model,
      label: model,
    }))
  }, [allModelsList, currentModelsArray])

  const modelMappingGuardrail = useMemo<ModelMappingGuardrail>(() => {
    if (!currentModelMapping?.trim()) {
      return createEmptyModelMappingGuardrail()
    }

    try {
      const parsed = JSON.parse(currentModelMapping)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
      }

      const entries = Object.entries(parsed).reduce<
        Array<{ source: string; target: string }>
      >((acc, [rawSource, rawTarget]) => {
        const source = String(rawSource).trim()
        const target = String(rawTarget ?? '').trim()

        if (!source || !target) {
          return acc
        }

        acc.push({ source, target })
        return acc
      }, [])

      const missingSourceModels = [
        ...new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.source) &&
                !currentModelsArray.includes(entry.source)
            )
            .map((entry) => entry.source)
        ),
      ]

      const exposedTargetModels = [
        ...new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.target) &&
                currentModelsArray.includes(entry.target)
            )
            .map((entry) => entry.target)
        ),
      ]

      return {
        invalidJson: false,
        entries,
        missingSourceModels,
        exposedTargetModels,
      }
    } catch {
      return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
    }
  }, [currentModelMapping, currentModelsArray])

  const mappingPreviewPairs =
    modelMappingGuardrail.entries.length > 0
      ? modelMappingGuardrail.entries.slice(0, 3)
      : MODEL_MAPPING_PREVIEW_FALLBACK
  const remainingMappingCount =
    modelMappingGuardrail.entries.length > 3
      ? modelMappingGuardrail.entries.length - 3
      : 0

  const upstreamUpdateMeta = useMemo(() => {
    const settings = parseSettingsRecord(currentSettings)
    const detectedModels = Array.isArray(
      settings.upstream_model_update_last_detected_models
    )
      ? settings.upstream_model_update_last_detected_models
          .map((model) => String(model || '').trim())
          .filter(Boolean)
      : []

    return {
      lastCheckTime: settings.upstream_model_update_last_check_time,
      detectedModels: [...new Set(detectedModels)],
    }
  }, [currentSettings])

  const upstreamDetectedModelsPreview = upstreamUpdateMeta.detectedModels.slice(
    0,
    UPSTREAM_DETECTED_MODEL_PREVIEW_LIMIT
  )
  const upstreamDetectedModelsOmittedCount =
    upstreamUpdateMeta.detectedModels.length -
    upstreamDetectedModelsPreview.length

  // Load channel data into form when editing
  useEffect(() => {
    if (isEditing && channelData?.data) {
      const defaults = transformChannelToFormDefaults(channelData.data)
      form.reset(defaults)
      setConfigurationSection('connection')
      // Store initial values for comparison
      initialModelsRef.current = parseModelsString(
        channelData.data.models || ''
      )
      initialModelMappingRef.current = channelData.data.model_mapping || ''
      initialStatusCodeMappingRef.current =
        channelData.data.status_code_mapping || ''
    } else if (!isEditing) {
      form.reset(CHANNEL_FORM_DEFAULT_VALUES)
      setConfigurationSection('connection')
      initialModelsRef.current = []
      initialModelMappingRef.current = ''
      initialStatusCodeMappingRef.current = ''
    }
  }, [isEditing, channelData, form])

  // Handle type change - set default values for specific types
  useEffect(() => {
    if (isEditing) return // Don't auto-set defaults when editing

    if (currentType === 60) {
      const seedanceDefaults = resolveSeedanceMaxChannelModelDefaults(
        currentType,
        form.getValues('models'),
        form.getValues('model_mapping')
      )
      form.setValue('models', seedanceDefaults.models)
      form.setValue('model_mapping', seedanceDefaults.modelMapping)
    }

    // Type 45 (VolcEngine) - set default base_url
    if (currentType === 45) {
      const currentBaseUrlValue = form.getValues('base_url')
      if (!currentBaseUrlValue || currentBaseUrlValue === '') {
        form.setValue('base_url', 'https://ark.cn-beijing.volces.com')
      }
    }

    // Type 18 (Xunfei) - set default other (version)
    if (currentType === 18) {
      const currentOther = form.getValues('other')
      if (!currentOther || currentOther === '') {
        form.setValue('other', 'v2.1')
      }
    }
  }, [currentType, isEditing, form])

  useEffect(() => {
    if (currentType !== 45 || currentBaseUrl !== 'doubao-coding-plan') return

    form.setValue('base_url', 'https://ark.cn-beijing.volces.com', {
      shouldDirty: false,
      shouldValidate: true,
    })
  }, [currentBaseUrl, currentType, form])

  useEffect(() => {
    if (isEditing || supportsMultiKeyAddMode) return
    if (multiKeyMode && multiKeyMode !== 'single') {
      form.setValue('multi_key_mode', 'single', {
        shouldDirty: true,
        shouldValidate: true,
      })
    }
  }, [form, isEditing, multiKeyMode, supportsMultiKeyAddMode])

  // Validate base_url - warn if it ends with /v1
  useEffect(() => {
    if (!currentBaseUrl || !currentBaseUrl.endsWith('/v1')) return

    // Show warning toast
    const timer = setTimeout(() => {
      toast.warning(
        t(
          'Warning: Base URL should not end with /v1. New API will handle it automatically. This may cause request failures.'
        ),
        { duration: 5000 }
      )
    }, 500)

    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentBaseUrl])

  // Handle key deduplication
  const handleDeduplicateKeys = () => {
    const currentKey = form.getValues('key')
    if (!currentKey || currentKey.trim() === '') {
      toast.info(t('Please enter keys first'))
      return
    }

    const result = deduplicateKeys(currentKey)

    if (result.removedCount === 0) {
      toast.info(t('No duplicate keys found'))
    } else {
      form.setValue('key', result.deduplicatedText)
      toast.success(
        t(
          'Removed {{removed}} duplicate key(s). Before: {{before}}, After: {{after}}',
          {
            removed: result.removedCount,
            before: result.beforeCount,
            after: result.afterCount,
          }
        )
      )
    }
  }

  const handleRefreshCodexCredential = useCallback(async () => {
    if (!channelId) return
    setIsCodexCredentialRefreshing(true)
    try {
      const res = await refreshCodexCredential(channelId)
      if (!res.success) {
        throw new Error(res.message || t('Failed to refresh credential'))
      }
      toast.success(t('Credential refreshed'))
      queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.detail(channelId),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setIsCodexCredentialRefreshing(false)
    }
  }, [channelId, queryClient, t])

  // Unified function to update models
  const updateModels = useCallback(
    (newModels: string[], merge: boolean = false) => {
      const finalModels = merge
        ? formatModelsArray([...currentModelsArray, ...newModels])
        : formatModelsArray(newModels)
      form.setValue('models', finalModels)
      return newModels.length
    },
    [currentModelsArray, form]
  )

  // Handle model operations
  const handleFillRelatedModels = useCallback(() => {
    if (!basicModels.length) {
      toast.info(t('No related models available for this channel type'))
      return
    }
    updateModels(basicModels)
    toast.success(
      t('Filled {{count}} related model(s)', { count: basicModels.length })
    )
  }, [basicModels, updateModels, t])

  const handleFillAllModels = useCallback(() => {
    if (!allModelsList.length) {
      toast.info(t('No models available'))
      return
    }
    updateModels(allModelsList)
    toast.success(
      t('Filled {{count}} model(s)', { count: allModelsList.length })
    )
  }, [allModelsList, updateModels, t])

  const handleClearModels = useCallback(() => {
    form.setValue('models', '')
    toast.success(t('Cleared all models'))
  }, [form, t])

  const handleCopyModels = useCallback(async () => {
    const models = form.getValues('models')
    if (!models?.trim()) {
      toast.info(t('No models to copy'))
      return
    }
    await copyToClipboard(models)
  }, [form, copyToClipboard, t])

  // Handle adding prefill group models
  const handleAddPrefillGroup = useCallback(
    (group: { id: number; name: string; items: string | string[] }) => {
      try {
        const items = Array.isArray(group.items)
          ? group.items
          : JSON.parse(group.items)

        if (!Array.isArray(items)) {
          throw new Error('Invalid items format')
        }

        const count = updateModels(items, true)
        toast.success(
          t('Added {{count}} models from "{{name}}"', {
            count,
            name: group.name,
          })
        )
      } catch {
        toast.error(t('Failed to parse group items'))
      }
    },
    [updateModels, t]
  )

  // Handle model selection change from MultiSelect
  const handleModelsChange = useCallback(
    (selected: string[]) => {
      form.setValue('models', selected.join(','))
    },
    [form]
  )

  // Handle successful submission
  const handleSuccess = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
    if (channelId) {
      queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.detail(channelId),
      })
    }
    onOpenChange(false)
    setOpen(null)
  }, [channelId, queryClient, onOpenChange, setOpen])

  // Show missing models confirmation dialog
  const confirmMissingModelMappings = useCallback(
    (missingModels: string[]): Promise<MissingModelsAction> => {
      return new Promise((resolve) => {
        setMissingModelsList(missingModels)
        setMissingModelsDialogOpen(true)
        missingModelsResolveRef.current = resolve
      })
    },
    []
  )

  // Handle missing models dialog action
  const handleMissingModelsAction = useCallback(
    (action: MissingModelsAction) => {
      setMissingModelsDialogOpen(false)
      if (missingModelsResolveRef.current) {
        missingModelsResolveRef.current(action)
        missingModelsResolveRef.current = null
      }
    },
    []
  )

  const confirmStatusCodeRisk = useCallback(
    (detailItems: string[]): Promise<boolean> =>
      new Promise((resolve) => {
        statusCodeRiskResolveRef.current = resolve
        setStatusCodeRiskDetailItems(detailItems)
        setStatusCodeRiskOpen(true)
      }),
    []
  )

  const handleStatusCodeRiskAction = useCallback((confirmed: boolean) => {
    setStatusCodeRiskOpen(false)
    setStatusCodeRiskDetailItems([])
    if (statusCodeRiskResolveRef.current) {
      statusCodeRiskResolveRef.current(confirmed)
      statusCodeRiskResolveRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => {
      if (statusCodeRiskResolveRef.current) {
        statusCodeRiskResolveRef.current(false)
        statusCodeRiskResolveRef.current = null
      }
    }
  }, [])

  const channelMutation = useChannelMutateForm({
    currentRow,
    isEditing,
    isMultiKeyChannel,
    onSuccess: handleSuccess,
  })

  const isSubmitting = channelMutation.isPending

  // Submit handler
  const onSubmit = useCallback(
    async (data: ChannelFormValues) => {
      if (isChannelDetailUnavailable) return
      // Validate key is required when creating
      if (!isEditing && !data.key?.trim()) {
        form.setError('key', {
          type: 'manual',
          message: ERROR_MESSAGES.REQUIRED_KEY,
        })
        return
      }

      if (sensitiveLocked) {
        const dirtyFields = form.formState.dirtyFields as Partial<
          Record<keyof ChannelFormValues, unknown>
        >
        const hasSensitiveChanges = SENSITIVE_FORM_FIELDS.some((field) =>
          Boolean(dirtyFields[field])
        )
        if (hasSensitiveChanges) {
          toast.error(
            t('You do not have permission to edit sensitive channel settings.')
          )
          return
        }
      }

      // Validate status_code_mapping entries
      if (data.status_code_mapping?.trim()) {
        const invalidEntries = collectInvalidStatusCodeEntries(
          data.status_code_mapping
        )
        if (invalidEntries.length > 0) {
          toast.error(
            t('Invalid status code mapping entries: {{entries}}', {
              entries: invalidEntries.join(', '),
            })
          )
          return
        }

        const riskyRedirects = collectNewDisallowedStatusCodeRedirects(
          initialStatusCodeMappingRef.current,
          data.status_code_mapping
        )
        if (riskyRedirects.length > 0) {
          const confirmed = await confirmStatusCodeRisk(riskyRedirects)
          if (!confirmed) return
        }
      }

      // Validate model_mapping JSON format
      const hasModelMapping =
        typeof data.model_mapping === 'string' &&
        data.model_mapping.trim() !== ''
      const modelMappingValue = data.model_mapping || ''

      if (hasModelMapping) {
        const validation = validateModelMappingJson(modelMappingValue)
        if (!validation.valid) {
          toast.error(t(validation.error || 'Invalid model mapping'))
          return
        }
      }

      // Normalize models array
      const normalizedModels = parseModelsString(data.models || '')

      // Check for missing models in model_mapping
      if (hasModelMapping) {
        const missingModels = findMissingModelsInMapping(
          modelMappingValue,
          normalizedModels
        )

        const shouldPromptMissing =
          missingModels.length > 0 &&
          hasModelConfigChanged(
            normalizedModels,
            data.model_mapping || '',
            initialModelsRef.current,
            initialModelMappingRef.current
          )

        if (shouldPromptMissing) {
          const confirmAction = await confirmMissingModelMappings(missingModels)
          if (confirmAction === 'cancel') {
            return
          }
          if (confirmAction === 'add') {
            const updatedModels = [
              ...new Set([...normalizedModels, ...missingModels]),
            ]
            data.models = formatModelsArray(updatedModels)
            form.setValue('models', data.models)
          }
        }
      }

      await channelMutation.mutateAsync(data)
    },
    [
      isEditing,
      isChannelDetailUnavailable,
      sensitiveLocked,
      form,
      confirmMissingModelMappings,
      confirmStatusCodeRisk,
      channelMutation,
      t,
    ]
  )

  useEffect(() => {
    if (!focusField) return
    const frame = window.requestAnimationFrame(() => {
      form.setFocus(focusField)
      setFocusField(null)
    })
    return () => window.cancelAnimationFrame(frame)
  }, [configurationSection, focusField, form])

  const onInvalid: SubmitErrorHandler<ChannelFormValues> = useCallback(
    (errors) => {
      const field = Object.keys(errors)[0] as
        | keyof ChannelFormValues
        | undefined
      if (field) {
        setConfigurationSection(getChannelConfigurationSectionForField(field))
        setFocusField(field)
      }
      toast.error(t('Please fix the highlighted fields before saving'))
    },
    [t]
  )

  // Handle drawer close
  const handleOpenChange = useCallback(
    (v: boolean) => {
      onOpenChange(v)
      if (!v) {
        form.reset(CHANNEL_FORM_DEFAULT_VALUES)
        setConfigurationSection('connection')
        setFocusField(null)
        setClipboardConnectionInfo(null)
      }
    },
    [onOpenChange, form]
  )

  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-5xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
              <div className='min-w-0'>
                <SheetTitle className='flex items-center gap-3'>
                  <IconBadge tone='info' size='title'>
                    <ChannelTypeLogo type={currentType} size={22} />
                  </IconBadge>
                  <span>
                    {isEditing ? t('Edit Channel') : t('Create Channel')}
                    <span className='text-muted-foreground ml-2 text-sm font-normal'>
                      {t(currentTypeLabel)}
                    </span>
                  </span>
                </SheetTitle>
                <SheetDescription className='mt-1'>
                  {isEditing
                    ? t(
                        "Update channel configuration and click save when you're done."
                      )
                    : t(
                        'Add a new channel by providing the necessary information.'
                      )}
                </SheetDescription>
              </div>
              {!isEditing && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='shrink-0'
                  onClick={pasteConnectionInfoFromClipboard}
                >
                  <ClipboardPaste className='size-4' />
                  <span>{t('Paste Connection Info')}</span>
                </Button>
              )}
            </div>
          </SheetHeader>

          {sensitiveLocked && (
            <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
              <AlertDescription>
                {t(
                  'Sensitive channel settings are read-only for your account.'
                )}{' '}
                {t(
                  'You can still edit non-sensitive operations fields such as models, groups, priority, and weight.'
                )}
              </AlertDescription>
            </Alert>
          )}

          {!isEditing && clipboardConnectionInfo && (
            <Alert>
              <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                <span>{t('Connection info detected in clipboard')}</span>
                <span className='flex shrink-0 gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    onClick={() => applyConnectionInfo(clipboardConnectionInfo)}
                  >
                    {t('Fill in')}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={() => setClipboardConnectionInfo(null)}
                  >
                    {t('Ignore')}
                  </Button>
                </span>
              </AlertDescription>
            </Alert>
          )}

          <Form {...form}>
            <form
              id='channel-form'
              ref={channelFormRef}
              onSubmit={(event) => {
                if (providerPickerOpen) {
                  event.preventDefault()
                  return
                }
                void form.handleSubmit(onSubmit, onInvalid)(event)
              }}
              className={sideDrawerFormClassName('gap-5 overflow-hidden')}
            >
              {isChannelDetailLoading && <ChannelEditorLoadingState />}
              {isChannelError && isChannelDetailUnavailable && (
                <ErrorState
                  title={t('Failed to load channel')}
                  description={getServerErrorMessage(
                    channelError,
                    t('Failed to load channel')
                  )}
                  onRetry={() => {
                    void refetchChannel()
                  }}
                />
              )}
              {!isChannelDetailLoading && !isChannelDetailUnavailable && (
                <>
                  {providerPickerOpen && (
                    <ChannelProviderPicker
                      currentType={currentType}
                      canBindTaskPlugin={canBindTaskPlugin}
                      disabled={sensitiveLocked || isSubmitting}
                      onSelect={(type) => {
                        form.setValue('type', type, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                        setProviderChosen(true)
                        setProviderPickerOpen(false)
                        setConfigurationSection('connection')
                      }}
                    />
                  )}
                  <div
                    className={
                      providerPickerOpen
                        ? 'hidden'
                        : 'flex min-h-0 flex-1 flex-col'
                    }
                  >
                    <ChannelConfiguration
                      section={configurationSection}
                      onSectionChange={setConfigurationSection}
                      statuses={configurationStatuses}
                      connection={
                        <>
                          <div
                            id={CHANNEL_EDITOR_SECTION_IDS.identity}
                            className='scroll-mt-4'
                          >
                            <ChannelBasicSection>
                              <div className='grid gap-4 sm:grid-cols-2'>
                                <fieldset
                                  disabled={sensitiveLocked}
                                  className='min-w-0 disabled:opacity-60'
                                >
                                  <FormField
                                    control={form.control}
                                    name='type'
                                    render={({ field }) => (
                                      <FormItem>
                                        <FormLabel>{t('Type *')}</FormLabel>
                                        <FormControl>
                                          <Button
                                            type='button'
                                            variant='outline'
                                            ref={field.ref}
                                            aria-label={t('Change provider')}
                                            disabled={
                                              sensitiveLocked || isSubmitting
                                            }
                                            onClick={() =>
                                              setProviderPickerOpen(true)
                                            }
                                            className='w-full justify-start'
                                          >
                                            <ChannelTypeLogo
                                              type={Number(field.value)}
                                              size={18}
                                            />
                                            <span className='min-w-0 flex-1 truncate text-start'>
                                              {t(currentTypeLabel)}
                                            </span>
                                            <span>{t('Change provider')}</span>
                                          </Button>
                                        </FormControl>
                                        {sensitiveLocked && (
                                          <FormDescription>
                                            {t(
                                              'No permission to perform this action'
                                            )}
                                          </FormDescription>
                                        )}
                                        <FormMessage />
                                      </FormItem>
                                    )}
                                  />
                                </fieldset>

                                <FormField
                                  control={form.control}
                                  name='name'
                                  render={({ field }) => (
                                    <FormItem>
                                      <FormLabel>{t('Name *')}</FormLabel>
                                      <FormControl>
                                        <Input
                                          placeholder={t(
                                            FIELD_PLACEHOLDERS.NAME
                                          )}
                                          {...field}
                                        />
                                      </FormControl>
                                      <FormMessage />
                                    </FormItem>
                                  )}
                                />
                              </div>

                              {currentType === CHANNEL_TYPE_TASK_PLUGIN && (
                                <FormField
                                  control={form.control}
                                  name='task_plugin_key'
                                  render={({ field }) => (
                                    <FormItem>
                                      <FormLabel>{t('Plugin key')}</FormLabel>
                                      <Select
                                        items={(taskPluginOptionsQuery.data ?? []).map((plugin) => ({
                                          value: plugin.key,
                                          label: plugin.name,
                                        }))}
                                        value={field.value || ''}
                                        onValueChange={(value) => {
                                          const previousPlugin = taskPluginOptionsQuery.data?.find((item) => item.key === field.value)
                                          field.onChange(value)
                                          const plugin = taskPluginOptionsQuery.data?.find((item) => item.key === value)
                                          if (!plugin || isEditing) return
                                          if (!form.getValues('name').trim()) {
                                            form.setValue('name', plugin.name, { shouldDirty: true })
                                          }
                                          form.setValue('models', plugin.models.join(','), { shouldDirty: true })
                                          const currentBaseUrl = form.getValues('base_url')?.trim()
                                          if (plugin.baseUrl && (!currentBaseUrl || currentBaseUrl === previousPlugin?.baseUrl)) {
                                            form.setValue('base_url', plugin.baseUrl, { shouldDirty: true })
                                          }
                                        }}
                                        disabled={!canBindTaskPlugin || sensitiveLocked || isSubmitting}
                                      >
                                        <FormControl>
                                          <SelectTrigger>
                                            <SelectValue placeholder={t('Select task plugin')} />
                                          </SelectTrigger>
                                        </FormControl>
                                        <SelectContent alignItemWithTrigger={false}>
                                          <SelectGroup>
                                            {(taskPluginOptionsQuery.data ?? []).map((plugin) => (
                                              <SelectItem key={plugin.key} value={plugin.key}>
                                                {plugin.name}
                                              </SelectItem>
                                            ))}
                                          </SelectGroup>
                                        </SelectContent>
                                      </Select>
                                      <FormDescription>
                                        {t('Selecting a plugin fills its declared models and default base URL.')}
                                      </FormDescription>
                                      <FormMessage />
                                    </FormItem>
                                  )}
                                />
                              )}

                              {!isEditing && (
                                <FormField
                                  control={form.control}
                                  name='status'
                                  render={({ field }) => (
                                    <FormItem
                                      className={sideDrawerSwitchItemClassName()}
                                    >
                                      <div className='flex flex-col gap-0.5'>
                                        <FormLabel>{t('Enabled')}</FormLabel>
                                        <FormDescription className='text-xs'>
                                          {t('Enable or disable this channel')}
                                        </FormDescription>
                                      </div>
                                      <FormControl>
                                        <Switch
                                          checked={field.value === 1}
                                          onCheckedChange={(checked) =>
                                            field.onChange(checked ? 1 : 2)
                                          }
                                        />
                                      </FormControl>
                                    </FormItem>
                                  )}
                                />
                              )}

                              {currentType === 1 && (
                                <fieldset
                                  disabled={sensitiveLocked}
                                  className='disabled:opacity-60'
                                >
                                  <FormField
                                    control={form.control}
                                    name='openai_organization'
                                    render={({ field }) => (
                                      <FormItem>
                                        <FormLabel>
                                          {t('OpenAI Organization')}
                                        </FormLabel>
                                        <FormControl>
                                          <Input
                                            placeholder={t('org-...')}
                                            {...field}
                                          />
                                        </FormControl>
                                        <FormDescription>
                                          {sensitiveLocked
                                            ? t(
                                                'No permission to perform this action'
                                              )
                                            : t(FIELD_DESCRIPTIONS.OPENAI_ORG)}
                                        </FormDescription>
                                        <FormMessage />
                                      </FormItem>
                                    )}
                                  />
                                </fieldset>
                              )}
                            </ChannelBasicSection>
                          </div>
                          <div
                            id={CHANNEL_EDITOR_SECTION_IDS.credentials}
                            className='scroll-mt-4'
                          >
                            <ChannelApiAccessSection>
                              {CHANNEL_TYPE_WARNINGS[currentType] && (
                                <Alert>
                                  <AlertDescription>
                                    {t(CHANNEL_TYPE_WARNINGS[currentType])}
                                  </AlertDescription>
                                </Alert>
                              )}

                              {sensitiveLocked && (
                                <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                                  <AlertDescription>
                                    {t('No permission to perform this action')}
                                  </AlertDescription>
                                </Alert>
                              )}

                              <div className='border-border/60 bg-muted/10 rounded-lg border p-4'>
                                <fieldset
                                  disabled={sensitiveLocked}
                                  className='space-y-4 disabled:opacity-60'
                                >
                                  {/* Azure (type 3) */}
                                  {currentType === 3 && (
                                    <>
                                      <FormField
                                        control={form.control}
                                        name='base_url'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('AZURE_OPENAI_ENDPOINT *')}
                                            </FormLabel>
                                            <FormControl>
                                              <Input
                                                placeholder={t(
                                                  'e.g., https://docs-test-001.openai.azure.com'
                                                )}
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              {t(
                                                'Your Azure OpenAI endpoint URL'
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                      <FormField
                                        control={form.control}
                                        name='other'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Default API Version *')}
                                            </FormLabel>
                                            <FormControl>
                                              <Input
                                                placeholder={t(
                                                  'e.g., 2025-04-01-preview'
                                                )}
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              {t(
                                                'Default API version for this channel'
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                      <FormField
                                        control={form.control}
                                        name='azure_responses_version'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Responses API Version')}
                                            </FormLabel>
                                            <FormControl>
                                              <Input
                                                placeholder={t('e.g., preview')}
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              {t(
                                                'Default Responses API version, if empty, will use the API version above'
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    </>
                                  )}

                                  {/* Custom (type 8) */}
                                  {currentType === 8 && (
                                    <FormField
                                      control={form.control}
                                      name='base_url'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Full Base URL (supports')} {'{'}
                                            {t('model')}
                                            {'}'} {t('variable) *')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t(
                                                'e.g., https://api.openai.com/v1/chat/completions'
                                              )}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t(
                                              'Enter the complete URL, supports'
                                            )}{' '}
                                            {'{'}
                                            {t('model')}
                                            {'}'} {t('variable')}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* Xunfei/Spark (type 18) */}
                                  {currentType === 18 && (
                                    <FormField
                                      control={form.control}
                                      name='other'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Model Version *')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t('e.g., v2.1')}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t(
                                              'Spark model version, e.g., v2.1 (version number in API URL)'
                                            )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* OpenRouter (type 20) */}
                                  {currentType === 20 && (
                                    <FormField
                                      control={form.control}
                                      name='is_enterprise_account'
                                      render={({ field }) => (
                                        <FormItem className='flex items-center justify-between'>
                                          <div className='space-y-0.5'>
                                            <FormLabel>
                                              {t('Enterprise Account')}
                                            </FormLabel>
                                            <FormDescription>
                                              {t(
                                                'Enable if this is an OpenRouter enterprise account with special response format'
                                              )}
                                            </FormDescription>
                                          </div>
                                          <FormControl>
                                            <Switch
                                              checked={field.value}
                                              onCheckedChange={field.onChange}
                                            />
                                          </FormControl>
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* AWS (type 33) */}
                                  {currentType === 33 && (
                                    <FormField
                                      control={form.control}
                                      name='aws_key_type'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('AWS Key Format')}
                                          </FormLabel>
                                          <Select
                                            items={[
                                              {
                                                value: 'ak_sk',
                                                label: t(
                                                  'AccessKey / SecretAccessKey'
                                                ),
                                              },
                                              {
                                                value: 'api_key',
                                                label: t('API Key'),
                                              },
                                            ]}
                                            onValueChange={field.onChange}
                                            value={field.value}
                                          >
                                            <FormControl>
                                              <SelectTrigger>
                                                <SelectValue
                                                  placeholder={t(
                                                    'Select key format'
                                                  )}
                                                />
                                              </SelectTrigger>
                                            </FormControl>
                                            <SelectContent
                                              alignItemWithTrigger={false}
                                            >
                                              <SelectGroup>
                                                <SelectItem value='ak_sk'>
                                                  {t(
                                                    'AccessKey / SecretAccessKey'
                                                  )}
                                                </SelectItem>
                                                <SelectItem value='api_key'>
                                                  {t('API Key')}
                                                </SelectItem>
                                              </SelectGroup>
                                            </SelectContent>
                                          </Select>
                                          <FormDescription>
                                            {field.value === 'api_key'
                                              ? t(
                                                  'API Key mode: use APIKey|Region'
                                                )
                                              : t(
                                                  'AK/SK mode: use AccessKey|SecretAccessKey|Region'
                                                )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* AI Proxy Library (type 21) */}
                                  {currentType === 21 && (
                                    <FormField
                                      control={form.control}
                                      name='other'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Knowledge Base ID *')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t('e.g., 123456')}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t('Enter the knowledge base ID')}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* FastGPT (type 22) */}
                                  {currentType === 22 && (
                                    <FormField
                                      control={form.control}
                                      name='base_url'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Private Deployment URL')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={
                                                defaultBaseURLs?.[
                                                  currentType
                                                ] ||
                                                t(
                                                  'e.g., https://fastgpt.run/api/openapi'
                                                )
                                              }
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t(
                                              'For private deployments, format: https://fastgpt.run/api/openapi'
                                            )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* SunoAPI (type 36) */}
                                  {currentType === 36 && (
                                    <FormField
                                      control={form.control}
                                      name='base_url'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t(
                                              'API Base URL (Important: Not Chat API) *'
                                            )}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t(
                                                'e.g., https://api.example.com (path before /suno)'
                                              )}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t(
                                              'Enter the path before /suno, usually just the domain'
                                            )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* Cloudflare Workers AI (type 39) */}
                                  {currentType === 39 && (
                                    <FormField
                                      control={form.control}
                                      name='other'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Account ID *')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t(
                                                'e.g., d6b5da8hk1awo8nap34ube6gh'
                                              )}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t('Your Cloudflare Account ID')}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* SiliconFlow (type 40) */}
                                  {currentType === 40 && (
                                    <Alert>
                                      <AlertDescription>
                                        {t('Referral link:')}{' '}
                                        <a
                                          href='https://cloud.siliconflow.cn/i/hij0YNTZ'
                                          target='_blank'
                                          rel='noopener noreferrer'
                                          className='text-primary underline'
                                        >
                                          {t(
                                            'https://cloud.siliconflow.cn/i/hij0YNTZ'
                                          )}
                                        </a>
                                      </AlertDescription>
                                    </Alert>
                                  )}

                                  {/* Vertex AI (type 41) */}
                                  {currentType === 41 && (
                                    <>
                                      <FormField
                                        control={form.control}
                                        name='vertex_key_type'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Vertex AI Key Format')}
                                            </FormLabel>
                                            <Select
                                              items={[
                                                {
                                                  value: 'json',
                                                  label: t('JSON'),
                                                },
                                                {
                                                  value: 'api_key',
                                                  label: t('API Key'),
                                                },
                                              ]}
                                              onValueChange={field.onChange}
                                              value={field.value}
                                            >
                                              <FormControl>
                                                <SelectTrigger>
                                                  <SelectValue />
                                                </SelectTrigger>
                                              </FormControl>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  <SelectItem value='json'>
                                                    {t('JSON')}
                                                  </SelectItem>
                                                  <SelectItem value='api_key'>
                                                    {t('API Key')}
                                                  </SelectItem>
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                            <FormDescription>
                                              {field.value === 'json'
                                                ? t(
                                                    'JSON format supports service account JSON files'
                                                  )
                                                : t(
                                                    'API Key mode (does not support batch creation)'
                                                  )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                      {vertexKeyType === 'json' && (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Service account JSON file(s)')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              type='file'
                                              accept='.json,application/json'
                                              multiple={isBatchMode}
                                              onChange={async (e) => {
                                                const fileList = e.target.files
                                                const files = fileList
                                                  ? [...fileList]
                                                  : []
                                                // allow re-selecting the same file
                                                e.target.value = ''

                                                if (files.length === 0) {
                                                  toast.info(
                                                    t(
                                                      'Please upload key file(s)'
                                                    )
                                                  )
                                                  return
                                                }

                                                const keys: unknown[] = []
                                                for (const file of files) {
                                                  try {
                                                    const txt =
                                                      await file.text()
                                                    keys.push(JSON.parse(txt))
                                                  } catch {
                                                    toast.error(
                                                      t(
                                                        'Failed to parse JSON file: {{name}}',
                                                        {
                                                          name: file.name,
                                                        }
                                                      )
                                                    )
                                                    return
                                                  }
                                                }

                                                if (keys.length === 0) {
                                                  toast.info(
                                                    t(
                                                      'Please upload key file(s)'
                                                    )
                                                  )
                                                  return
                                                }

                                                const keyValue = isBatchMode
                                                  ? JSON.stringify(keys)
                                                  : JSON.stringify(keys[0])

                                                form.setValue('key', keyValue, {
                                                  shouldDirty: true,
                                                  shouldValidate: true,
                                                })

                                                toast.success(
                                                  t(
                                                    'Parsed {{count}} service account file(s)',
                                                    {
                                                      count: keys.length,
                                                    }
                                                  )
                                                )
                                              }}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {isBatchMode
                                              ? t(
                                                  'Upload multiple JSON files in batch modes'
                                                )
                                              : t(
                                                  'Upload a single service account JSON file'
                                                )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                      <FormField
                                        control={form.control}
                                        name='other'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Deployment Region *')}
                                            </FormLabel>
                                            <FormControl>
                                              <Textarea
                                                placeholder={t(
                                                  'e.g., us-central1 or JSON format for model-specific regions'
                                                )}
                                                rows={3}
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              {t(
                                                'Enter deployment region or JSON mapping:'
                                              )}{' '}
                                              {'{'}
                                              {t(
                                                '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"'
                                              )}
                                              {'}'}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    </>
                                  )}

                                  {/* VolcEngine (type 45) */}
                                  {currentType === 45 &&
                                    !doubaoApiEditUnlocked && (
                                      <FormField
                                        control={form.control}
                                        name='base_url'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel
                                              className='cursor-pointer select-none'
                                              onClick={
                                                handleApiConfigSecretClick
                                              }
                                            >
                                              {t('API Base URL *')}
                                            </FormLabel>
                                            <Select
                                              items={[
                                                {
                                                  value:
                                                    'https://ark.cn-beijing.volces.com',
                                                  label: t(
                                                    'https://ark.cn-beijing.volces.com'
                                                  ),
                                                },
                                                {
                                                  value:
                                                    'https://ark.ap-southeast.bytepluses.com',
                                                  label: t(
                                                    'https://ark.ap-southeast.bytepluses.com'
                                                  ),
                                                },
                                              ]}
                                              onValueChange={field.onChange}
                                              value={
                                                field.value ===
                                                'doubao-coding-plan'
                                                  ? 'https://ark.cn-beijing.volces.com'
                                                  : field.value ||
                                                    'https://ark.cn-beijing.volces.com'
                                              }
                                            >
                                              <FormControl>
                                                <SelectTrigger>
                                                  <SelectValue />
                                                </SelectTrigger>
                                              </FormControl>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  <SelectItem value='https://ark.cn-beijing.volces.com'>
                                                    {t(
                                                      'https://ark.cn-beijing.volces.com'
                                                    )}
                                                  </SelectItem>
                                                  <SelectItem value='https://ark.ap-southeast.bytepluses.com'>
                                                    {t(
                                                      'https://ark.ap-southeast.bytepluses.com'
                                                    )}
                                                  </SelectItem>
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                            <FormDescription>
                                              {t(
                                                'Select the API endpoint region'
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    )}

                                  {/* VolcEngine (type 45) - Custom API URL (unlocked) */}
                                  {currentType === 45 &&
                                    doubaoApiEditUnlocked && (
                                      <FormField
                                        control={form.control}
                                        name='base_url'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('API Base URL *')}
                                            </FormLabel>
                                            <FormControl>
                                              <Input
                                                placeholder={
                                                  defaultBaseURLs?.[
                                                    currentType
                                                  ] ||
                                                  t(
                                                    'e.g., https://ark.cn-beijing.volces.com'
                                                  )
                                                }
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              {t(
                                                'Enter custom API endpoint URL'
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    )}

                                  {/* Coze (type 49) */}
                                  {currentType === 49 && (
                                    <FormField
                                      control={form.control}
                                      name='other'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>
                                            {t('Agent ID *')}
                                          </FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={t(
                                                'e.g., 7342866812345'
                                              )}
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t('Enter the Coze agent ID')}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {/* General base_url for other types */}
                                  {![3, 8, 22, 36, 45].includes(
                                    currentType
                                  ) && (
                                    <FormField
                                      control={form.control}
                                      name='base_url'
                                      render={({ field }) => (
                                        <FormItem>
                                          <FormLabel>{t('Base URL')}</FormLabel>
                                          <FormControl>
                                            <Input
                                              placeholder={
                                                defaultBaseURLs?.[
                                                  currentType
                                                ] ||
                                                t(FIELD_PLACEHOLDERS.BASE_URL)
                                              }
                                              {...field}
                                            />
                                          </FormControl>
                                          <FormDescription>
                                            {t(
                                              'Custom API base URL. For official channels, New API has built-in addresses. Only fill this for third-party proxy sites or special endpoints. Do not add /v1 or trailing slash.'
                                            )}
                                          </FormDescription>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  {currentType ===
                                    CHANNEL_TYPE_ADVANCED_CUSTOM && (
                                    <FormField
                                      control={form.control}
                                      name='advanced_custom'
                                      render={({ field }) => (
                                        <FormItem className='space-y-3 border-y py-4'>
                                          <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
                                            <div className='space-y-2'>
                                              <FormLabel>
                                                {t('Advanced Custom Routes')}
                                              </FormLabel>
                                              <div className='flex flex-wrap gap-2'>
                                                <Badge variant='secondary'>
                                                  {t('Routes')}:{' '}
                                                  {
                                                    advancedCustomStats.routeCount
                                                  }
                                                </Badge>
                                                {advancedCustomRouteTypeLabels.map(
                                                  (label) => (
                                                    <Badge
                                                      key={label}
                                                      variant='outline'
                                                      className='max-w-[12rem]'
                                                      title={label}
                                                    >
                                                      <span className='truncate'>
                                                        {label}
                                                      </span>
                                                    </Badge>
                                                  )
                                                )}
                                                {hiddenAdvancedCustomRouteTypeCount >
                                                  0 && (
                                                  <Badge
                                                    variant='outline'
                                                    title={
                                                      advancedCustomRouteTypeTitle
                                                    }
                                                  >
                                                    +
                                                    {
                                                      hiddenAdvancedCustomRouteTypeCount
                                                    }
                                                  </Badge>
                                                )}
                                                {!advancedCustomStats.valid && (
                                                  <Badge variant='destructive'>
                                                    {t('Incomplete')}
                                                  </Badge>
                                                )}
                                              </div>
                                            </div>
                                            <Button
                                              type='button'
                                              variant='outline'
                                              size='sm'
                                              onClick={() =>
                                                setAdvancedCustomEditorOpen(
                                                  true
                                                )
                                              }
                                            >
                                              <Route className='mr-2 h-4 w-4' />
                                              {t('Configure routes')}
                                            </Button>
                                          </div>
                                          <FormControl>
                                            <input type='hidden' {...field} />
                                          </FormControl>
                                          <FormMessage />
                                        </FormItem>
                                      )}
                                    />
                                  )}

                                  <ChannelAuthSection>
                                    {!isEditing && (
                                      <FormField
                                        control={form.control}
                                        name='multi_key_mode'
                                        render={({ field }) => (
                                          <FormItem className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                                            <FormLabel className='text-muted-foreground text-xs font-medium'>
                                              {t('Add Mode')}
                                            </FormLabel>
                                            <Select
                                              items={addModeOptions.map(
                                                (option) => ({
                                                  value: option.value,
                                                  label: t(option.label),
                                                })
                                              )}
                                              onValueChange={field.onChange}
                                              value={field.value}
                                            >
                                              <FormControl>
                                                <SelectTrigger
                                                  size='sm'
                                                  className='w-full sm:w-56'
                                                >
                                                  <SelectValue />
                                                </SelectTrigger>
                                              </FormControl>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  {addModeOptions.map(
                                                    (option) => (
                                                      <SelectItem
                                                        key={option.value}
                                                        value={option.value}
                                                      >
                                                        {t(option.label)}
                                                      </SelectItem>
                                                    )
                                                  )}
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    )}

                                    <FormField
                                      control={form.control}
                                      name='key'
                                      render={({ field }) => {
                                        let keyPlaceholder = t(
                                          getKeyPromptForType(currentType)
                                        )
                                        if (isEditing) {
                                          keyPlaceholder = t(
                                            'Leave empty to keep existing key'
                                          )
                                        } else if (
                                          currentType === 33 &&
                                          awsKeyType === 'api_key' &&
                                          isBatchMode
                                        ) {
                                          keyPlaceholder = t(
                                            'Enter API Key, one per line, format: APIKey|Region'
                                          )
                                        } else if (
                                          currentType === 33 &&
                                          awsKeyType === 'api_key'
                                        ) {
                                          keyPlaceholder = t(
                                            'Enter API Key, format: APIKey|Region'
                                          )
                                        } else if (
                                          currentType === 33 &&
                                          isBatchMode
                                        ) {
                                          keyPlaceholder = t(
                                            'Enter key, one per line, format: AccessKey|SecretAccessKey|Region'
                                          )
                                        } else if (currentType === 33) {
                                          keyPlaceholder = t(
                                            'Enter key, format: AccessKey|SecretAccessKey|Region'
                                          )
                                        } else if (isBatchMode) {
                                          keyPlaceholder = t(
                                            'Enter one key per line for batch creation'
                                          )
                                        }

                                        let keyDescription: ReactNode = t(
                                          FIELD_DESCRIPTIONS.KEY
                                        )
                                        if (isEditing) {
                                          let keyModeDescription = t(
                                            'Append mode: New keys will be added to the end of the existing key list'
                                          )
                                          if (keyMode === 'replace') {
                                            keyModeDescription = t(
                                              'Replace mode: Will completely replace all existing keys'
                                            )
                                          }
                                          keyDescription = (
                                            <>
                                              {t(
                                                'Enter new key to update, or leave empty to keep current key'
                                              )}
                                              {isMultiKeyChannel && (
                                                <span className='text-warning mt-1 block'>
                                                  {keyModeDescription}
                                                </span>
                                              )}
                                            </>
                                          )
                                        } else if (isBatchMode) {
                                          keyDescription = t(
                                            'Enter one API key per line for batch creation'
                                          )
                                        }
                                        return (
                                          <FormItem>
                                            <FormLabel>
                                              {t('API Key *')}
                                            </FormLabel>
                                            <FormControl>
                                              <Textarea
                                                placeholder={keyPlaceholder}
                                                rows={isBatchMode ? 8 : 4}
                                                {...field}
                                              />
                                            </FormControl>
                                            <FormDescription>
                                              <span className='flex flex-col gap-2'>
                                                <span>{keyDescription}</span>
                                                {isBatchMode && (
                                                  <Button
                                                    type='button'
                                                    variant='outline'
                                                    size='sm'
                                                    onClick={
                                                      handleDeduplicateKeys
                                                    }
                                                    className='w-fit'
                                                  >
                                                    <Trash2 className='mr-2 h-4 w-4' />
                                                    {t('Remove Duplicates')}
                                                  </Button>
                                                )}
                                              </span>
                                            </FormDescription>
                                            {isEditing &&
                                              canRevealChannelKey && (
                                                <div className='border-border/60 mt-4 flex flex-col gap-3 border-y border-dashed py-4'>
                                                  <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                                                    <div>
                                                      <p className='text-sm font-medium'>
                                                        {t('Current key')}
                                                      </p>
                                                      <p className='text-muted-foreground text-xs'>
                                                        {t(
                                                          'Verification required to reveal the saved key.'
                                                        )}
                                                      </p>
                                                    </div>
                                                    <div className='flex items-center gap-2'>
                                                      <Button
                                                        type='button'
                                                        variant='outline'
                                                        size='sm'
                                                        onClick={
                                                          handleRevealKey
                                                        }
                                                        disabled={
                                                          isChannelKeyLoading ||
                                                          verification.isActive
                                                        }
                                                      >
                                                        {isChannelKeyLoading ||
                                                        verification.isActive ? (
                                                          <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                                                        ) : (
                                                          <Eye className='mr-2 h-4 w-4' />
                                                        )}
                                                        {t('Reveal key')}
                                                      </Button>
                                                      <Button
                                                        type='button'
                                                        variant='ghost'
                                                        size='sm'
                                                        onClick={async () => {
                                                          if (channelKey) {
                                                            await copyToClipboard(
                                                              channelKey
                                                            )
                                                          }
                                                        }}
                                                        disabled={!channelKey}
                                                      >
                                                        <Copy className='mr-2 h-4 w-4' />
                                                        {t('Copy')}
                                                      </Button>
                                                    </div>
                                                  </div>
                                                  <Input
                                                    readOnly
                                                    value={channelKey ?? ''}
                                                    placeholder={t(
                                                      'Hidden — verify to reveal'
                                                    )}
                                                    className='font-mono'
                                                  />
                                                </div>
                                              )}
                                            <FormMessage />
                                          </FormItem>
                                        )
                                      }}
                                    />

                                    {currentType === 57 && (
                                      <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
                                        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                                          <div className='text-muted-foreground text-xs'>
                                            {t(
                                              'Codex channels use an OAuth JSON credential as the key.'
                                            )}
                                          </div>
                                          <div className='flex flex-wrap items-center gap-2'>
                                            {isEditing && channelId && (
                                              <Button
                                                type='button'
                                                variant='outline'
                                                size='sm'
                                                onClick={
                                                  handleRefreshCodexCredential
                                                }
                                                disabled={
                                                  sensitiveLocked ||
                                                  isCodexCredentialRefreshing
                                                }
                                              >
                                                {isCodexCredentialRefreshing ? (
                                                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                                                ) : (
                                                  <RefreshCw className='mr-2 h-4 w-4' />
                                                )}
                                                {isCodexCredentialRefreshing
                                                  ? t('Refreshing...')
                                                  : t('Refresh credential')}
                                              </Button>
                                            )}
                                          </div>
                                        </div>
                                        <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                                          <AlertDescription>
                                            {t(
                                              "Disclaimer: Personal use only. Do not distribute or share any credentials. This channel has prerequisites and requires prior setup; use it only if you understand the flow and risks, and comply with OpenAI's terms and policies. Credentials and configuration are for Codex CLI integration only, and are not intended for any other client, platform, or channel."
                                            )}
                                          </AlertDescription>
                                        </Alert>
                                      </div>
                                    )}

                                    {isEditing && isMultiKeyChannel && (
                                      <FormField
                                        control={form.control}
                                        name='key_mode'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Key Update Mode')}
                                            </FormLabel>
                                            <Select
                                              items={[
                                                {
                                                  value: 'append',
                                                  label: t(
                                                    'Append to existing keys'
                                                  ),
                                                },
                                                {
                                                  value: 'replace',
                                                  label: t(
                                                    'Replace all existing keys'
                                                  ),
                                                },
                                              ]}
                                              onValueChange={field.onChange}
                                              value={field.value}
                                            >
                                              <FormControl>
                                                <SelectTrigger>
                                                  <SelectValue />
                                                </SelectTrigger>
                                              </FormControl>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  <SelectItem value='append'>
                                                    {t(
                                                      'Append to existing keys'
                                                    )}
                                                  </SelectItem>
                                                  <SelectItem value='replace'>
                                                    {t(
                                                      'Replace all existing keys'
                                                    )}
                                                  </SelectItem>
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                            <FormDescription>
                                              {field.value === 'replace'
                                                ? t(
                                                    'Replace mode: Will completely replace all existing keys'
                                                  )
                                                : t(
                                                    'Append mode: New keys will be added to the end of the existing key list'
                                                  )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    )}

                                    {(isMultiKeyChannel ||
                                      (!isEditing &&
                                        multiKeyMode ===
                                          'multi_to_single')) && (
                                      <FormField
                                        control={form.control}
                                        name='multi_key_type'
                                        render={({ field }) => (
                                          <FormItem>
                                            <FormLabel>
                                              {t('Multi-Key Strategy')}
                                            </FormLabel>
                                            <Select
                                              items={[
                                                {
                                                  value: 'random',
                                                  label: t('Random'),
                                                },
                                                {
                                                  value: 'polling',
                                                  label: t('Polling'),
                                                },
                                              ]}
                                              onValueChange={field.onChange}
                                              value={field.value}
                                            >
                                              <FormControl>
                                                <SelectTrigger>
                                                  <SelectValue />
                                                </SelectTrigger>
                                              </FormControl>
                                              <SelectContent
                                                alignItemWithTrigger={false}
                                              >
                                                <SelectGroup>
                                                  <SelectItem value='random'>
                                                    {t('Random')}
                                                  </SelectItem>
                                                  <SelectItem value='polling'>
                                                    {t('Polling')}
                                                  </SelectItem>
                                                </SelectGroup>
                                              </SelectContent>
                                            </Select>
                                            <FormDescription>
                                              {multiKeyType === 'polling' ? (
                                                <span className='text-warning'>
                                                  {t(
                                                    'Polling mode requires Redis and memory cache, otherwise performance will be significantly degraded'
                                                  )}
                                                </span>
                                              ) : (
                                                t(
                                                  'Randomly select a key from the pool for each request'
                                                )
                                              )}
                                            </FormDescription>
                                            <FormMessage />
                                          </FormItem>
                                        )}
                                      />
                                    )}
                                  </ChannelAuthSection>
                                </fieldset>
                              </div>
                            </ChannelApiAccessSection>
                          </div>
                        </>
                      }
                      models={
                        <div
                          id={CHANNEL_EDITOR_SECTION_IDS.models}
                          className='scroll-mt-4'
                        >
                          <ChannelModelsSection>
                            <div className='space-y-5'>
                              <div className='border-border/60 bg-muted/10 rounded-lg border p-4'>
                                <FormField
                                  control={form.control}
                                  name='models'
                                  render={() => (
                                    <FormItem className='space-y-3'>
                                      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                                        <div className='space-y-1'>
                                          <FormLabel>{t('Models *')}</FormLabel>
                                          <FormDescription>
                                            {t(FIELD_DESCRIPTIONS.MODELS)}
                                          </FormDescription>
                                        </div>
                                        <Badge
                                          variant='outline'
                                          className='w-fit'
                                        >
                                          {t('Selected {{count}}', {
                                            count: currentModelsArray.length,
                                          })}
                                        </Badge>
                                      </div>
                                      <FormControl>
                                        <MultiSelect
                                          options={modelOptions}
                                          selected={currentModelsArray}
                                          onChange={handleModelsChange}
                                          placeholder={t(
                                            'Select models or add custom ones'
                                          )}
                                          allowCreate
                                          createLabel='Add custom model "{{value}}"'
                                          maxVisibleChips={8}
                                          copyChipOnClick
                                        />
                                      </FormControl>
                                      {modelMappingGuardrail.exposedTargetModels
                                        .length > 0 && (
                                        <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                                          <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                                            <span>
                                              {t(
                                                'The mapped upstream model(s)'
                                              )}{' '}
                                              {formatModelNames(
                                                modelMappingGuardrail.exposedTargetModels
                                              )}{' '}
                                              {t(
                                                'are also listed here. Remove them from Models to keep the `/v1/models` response user-friendly and hide vendor-specific names.'
                                              )}
                                            </span>
                                            <Button
                                              type='button'
                                              variant='outline'
                                              size='sm'
                                              onClick={() => {
                                                const hiddenTargets = new Set(
                                                  modelMappingGuardrail.exposedTargetModels
                                                )
                                                updateModels(
                                                  currentModelsArray.filter(
                                                    (model) =>
                                                      !hiddenTargets.has(model)
                                                  )
                                                )
                                              }}
                                            >
                                              {t('Remove mapped targets')}
                                            </Button>
                                          </AlertDescription>
                                        </Alert>
                                      )}
                                      <FormMessage />
                                    </FormItem>
                                  )}
                                />

                                <Separator className='my-4' />

                                <div className='space-y-3'>
                                  <div>
                                    <p className='text-sm font-medium'>
                                      {t('Quick actions')}
                                    </p>
                                    <p className='text-muted-foreground text-xs'>
                                      {t(
                                        'Use presets or upstream discovery to populate the model list faster.'
                                      )}
                                    </p>
                                  </div>
                                  <div className='flex flex-wrap gap-2'>
                                    <Button
                                      type='button'
                                      variant='outline'
                                      size='sm'
                                      onClick={handleFillRelatedModels}
                                      disabled={!basicModels.length}
                                    >
                                      <FileText
                                        className='mr-2 h-4 w-4'
                                        aria-hidden='true'
                                      />
                                      {t('Fill Related Models')}
                                    </Button>
                                    <Button
                                      type='button'
                                      variant='outline'
                                      size='sm'
                                      onClick={handleFillAllModels}
                                      disabled={!allModelsList.length}
                                    >
                                      <Plus
                                        className='mr-2 h-4 w-4'
                                        aria-hidden='true'
                                      />
                                      {t('Fill All Models')}
                                    </Button>
                                    <Button
                                      type='button'
                                      variant='outline'
                                      size='sm'
                                      onClick={handleCopyModels}
                                      disabled={currentModelsArray.length === 0}
                                    >
                                      <Copy
                                        className='mr-2 h-4 w-4'
                                        aria-hidden='true'
                                      />
                                      {t('Copy All')}
                                    </Button>
                                    <Button
                                      type='button'
                                      variant='ghost'
                                      size='sm'
                                      onClick={handleClearModels}
                                      disabled={currentModelsArray.length === 0}
                                    >
                                      <Eraser
                                        className='mr-2 h-4 w-4'
                                        aria-hidden='true'
                                      />
                                      {t('Clear All')}
                                    </Button>
                                  </div>
                                  {prefillGroups.length > 0 && (
                                    <div className='flex flex-wrap items-center gap-2'>
                                      <span className='text-muted-foreground text-xs'>
                                        {t('Preset groups')}:
                                      </span>
                                      {prefillGroups.map((group) => (
                                        <Button
                                          key={group.id}
                                          type='button'
                                          variant='secondary'
                                          size='sm'
                                          onClick={() =>
                                            handleAddPrefillGroup(group)
                                          }
                                        >
                                          {group.name}
                                        </Button>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              </div>

                              {MODEL_FETCHABLE_TYPES.has(currentType) && (
                                <ChannelModelDiscovery
                                  request={{
                                    type: currentType,
                                    channel_id:
                                      isEditing &&
                                      currentType === currentRow?.type
                                        ? (channelId ?? undefined)
                                        : undefined,
                                    key: currentKey,
                                    base_url: currentBaseUrl || '',
                                    advanced_custom:
                                      currentType ===
                                      CHANNEL_TYPE_ADVANCED_CUSTOM
                                        ? currentAdvancedCustom
                                        : undefined,
                                    header_override:
                                      currentHeaderOverride || '',
                                    proxy: currentProxy || '',
                                  }}
                                  enabled={
                                    open &&
                                    (canEditSensitive ||
                                      (isEditing && canOperateChannel))
                                  }
                                  savedChannelId={
                                    sensitiveLocked
                                      ? (channelId ?? undefined)
                                      : undefined
                                  }
                                  selected={currentModelsArray}
                                  existingModels={initialModelsRef.current}
                                  redirectModels={redirectModelList}
                                  redirectSourceModels={redirectModelKeyList}
                                  onChange={(models) =>
                                    form.setValue(
                                      'models',
                                      formatModelsArray(models),
                                      {
                                        shouldDirty: true,
                                        shouldValidate: true,
                                      }
                                    )
                                  }
                                />
                              )}

                              <div className='border-border/60 rounded-lg border p-4'>
                                <FormField
                                  control={form.control}
                                  name='group'
                                  render={({ field }) => (
                                    <FormItem className='space-y-3'>
                                      <div className='space-y-1'>
                                        <FormLabel>{t('Groups *')}</FormLabel>
                                        <FormDescription>
                                          {t(FIELD_DESCRIPTIONS.GROUP)}
                                        </FormDescription>
                                      </div>
                                      <FormControl>
                                        {isLoadingGroups ? (
                                          <Skeleton className='h-10 w-full' />
                                        ) : (
                                          <MultiSelect
                                            options={groupOptions}
                                            selected={field.value}
                                            onChange={field.onChange}
                                            placeholder={t(
                                              FIELD_PLACEHOLDERS.GROUP
                                            )}
                                          />
                                        )}
                                      </FormControl>
                                      <FormMessage />
                                    </FormItem>
                                  )}
                                />
                              </div>
                            </div>
                          </ChannelModelsSection>
                        </div>
                      }
                      routing={
                        <>
                          <div className='border-border/60 rounded-lg border p-4'>
                            <FormField
                              control={form.control}
                              name='model_mapping'
                              render={({ field }) => (
                                <FormItem className='space-y-3'>
                                  <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                                    <div className='space-y-1'>
                                      <div className='flex items-center gap-2'>
                                        <FormLabel className='mb-0'>
                                          {t('Model Mapping')}
                                        </FormLabel>
                                        <Popover>
                                          <PopoverTrigger
                                            render={
                                              <Button
                                                type='button'
                                                variant='ghost'
                                                size='icon-sm'
                                                className='text-muted-foreground hover:text-foreground size-auto p-0'
                                                aria-label={t(
                                                  'How model mapping works'
                                                )}
                                              />
                                            }
                                          >
                                            <HelpCircle
                                              className='h-4 w-4'
                                              aria-hidden='true'
                                            />
                                          </PopoverTrigger>
                                          <PopoverContent
                                            side='top'
                                            align='start'
                                            className='w-96 max-w-[calc(100vw-2rem)] space-y-2 text-left'
                                          >
                                            <PopoverTitle>
                                              {t('Request flow')}
                                            </PopoverTitle>
                                            <div className='space-y-1 font-mono text-xs'>
                                              {mappingPreviewPairs.map(
                                                (pair) => (
                                                  <div
                                                    key={`${pair.source}-${pair.target}`}
                                                    className='flex items-center gap-1'
                                                  >
                                                    <span className='min-w-0 flex-1 wrap-anywhere'>
                                                      {pair.source}
                                                    </span>
                                                    <ArrowRight
                                                      className='h-3.5 w-3.5 opacity-70'
                                                      aria-hidden='true'
                                                    />
                                                    <span className='min-w-0 flex-1 wrap-anywhere'>
                                                      {pair.target}
                                                    </span>
                                                  </div>
                                                )
                                              )}
                                              {remainingMappingCount > 0 && (
                                                <div className='text-[11px] opacity-70'>
                                                  +{remainingMappingCount}{' '}
                                                  {t('more mapping')}
                                                  {remainingMappingCount > 1
                                                    ? 's'
                                                    : ''}
                                                </div>
                                              )}
                                            </div>
                                            <p className='text-[11px] leading-relaxed opacity-80'>
                                              {t(
                                                'Users call the model on the left. The platform forwards the request to the upstream model on the right.'
                                              )}
                                            </p>
                                          </PopoverContent>
                                        </Popover>
                                      </div>
                                      <FormDescription>
                                        {t(FIELD_DESCRIPTIONS.MODEL_MAPPING)}
                                      </FormDescription>
                                    </div>
                                  </div>
                                  <FormControl>
                                    <ModelMappingEditor
                                      value={field.value || ''}
                                      onChange={field.onChange}
                                      disabled={isSubmitting}
                                      sourceModelOptions={currentModelsArray}
                                      targetModelOptions={modelOptions.map(
                                        (option) => option.value
                                      )}
                                    />
                                  </FormControl>
                                  {modelMappingGuardrail.invalidJson && (
                                    <Alert variant='destructive'>
                                      <AlertDescription>
                                        {t(
                                          'Model Mapping must be a JSON object like'
                                        )}{' '}
                                        <code className='font-mono'>
                                          {'{"gpt-4":"Azure-GPT4"}'}
                                        </code>
                                        {t(
                                          '. Please fix the JSON before saving.'
                                        )}
                                      </AlertDescription>
                                    </Alert>
                                  )}
                                  {modelMappingGuardrail.missingSourceModels
                                    .length > 0 && (
                                    <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                                      <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                                        <span>
                                          {t('Add')}{' '}
                                          {formatModelNames(
                                            modelMappingGuardrail.missingSourceModels
                                          )}{' '}
                                          {t(
                                            'to the Models list so users can use them before the mapping sends traffic upstream.'
                                          )}
                                        </span>
                                        <Button
                                          type='button'
                                          variant='outline'
                                          size='sm'
                                          onClick={() => {
                                            updateModels([
                                              ...currentModelsArray,
                                              ...modelMappingGuardrail.missingSourceModels,
                                            ])
                                          }}
                                        >
                                          {t('Add missing models')}
                                        </Button>
                                      </AlertDescription>
                                    </Alert>
                                  )}
                                  <FormMessage />
                                </FormItem>
                              )}
                            />
                          </div>
                          <div
                            id={ADVANCED_SETTINGS_SECTION_IDS.routingStrategy}
                            className={configuredAdvancedSectionClassName(
                              'flex scroll-mt-4 flex-col gap-4',
                              routingStrategyConfigured
                            )}
                          >
                            <SubHeading
                              title={t('Routing Strategy')}
                              icon={<Route className='h-3.5 w-3.5' />}
                              iconTone='info'
                            />
                            <div className='grid gap-4 sm:grid-cols-2'>
                              <FormField
                                control={form.control}
                                name='priority'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('Priority')}</FormLabel>
                                    <FormControl>
                                      <Input
                                        type='number'
                                        placeholder='0'
                                        {...field}
                                        onChange={(e) =>
                                          field.onChange(Number(e.target.value))
                                        }
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(FIELD_DESCRIPTIONS.PRIORITY)}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='weight'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('Weight')}</FormLabel>
                                    <FormControl>
                                      <Input
                                        type='number'
                                        placeholder='0'
                                        {...field}
                                        onChange={(e) =>
                                          field.onChange(Number(e.target.value))
                                        }
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(FIELD_DESCRIPTIONS.WEIGHT)}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />
                            </div>

                            <FormField
                              control={form.control}
                              name='test_model'
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>{t('Test Model')}</FormLabel>
                                  <FormControl>
                                    <Input
                                      placeholder={t(
                                        FIELD_PLACEHOLDERS.TEST_MODEL
                                      )}
                                      {...field}
                                    />
                                  </FormControl>
                                  <FormDescription>
                                    {t(FIELD_DESCRIPTIONS.TEST_MODEL)}
                                  </FormDescription>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            <FormField
                              control={form.control}
                              name='auto_ban'
                              render={({ field }) => (
                                <FormItem className='flex items-center justify-between'>
                                  <div className='space-y-0.5'>
                                    <FormLabel>{t('Auto Ban')}</FormLabel>
                                    <FormDescription>
                                      {t(FIELD_DESCRIPTIONS.AUTO_BAN)}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <Switch
                                      checked={field.value === 1}
                                      onCheckedChange={(checked) =>
                                        field.onChange(checked ? 1 : 0)
                                      }
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                          </div>
                        </>
                      }
                      request={
                        <>
                          <div
                            id={ADVANCED_SETTINGS_SECTION_IDS.overrideRules}
                            className={configuredAdvancedSectionClassName(
                              'flex scroll-mt-4 flex-col gap-4 border-t pt-4',
                              overrideRulesConfigured
                            )}
                          >
                            <SubHeading
                              title={t('Override Rules')}
                              icon={<Code className='h-3.5 w-3.5' />}
                              iconTone='chart-4'
                            />

                            <FormField
                              control={form.control}
                              name='status_code_mapping'
                              render={({ field }) => (
                                <FormItem className='space-y-3'>
                                  <div className='space-y-1'>
                                    <FormLabel>
                                      {t('Status Code Mapping')}
                                    </FormLabel>
                                    <FormDescription>
                                      {t(
                                        'Map upstream status codes to different codes'
                                      )}
                                    </FormDescription>
                                  </div>
                                  <FormControl>
                                    <JsonEditor
                                      value={field.value || ''}
                                      onChange={field.onChange}
                                      disabled={isSubmitting}
                                      keyPlaceholder='400'
                                      valuePlaceholder='500'
                                      keyLabel='Original Code'
                                      valueLabel='Mapped Code'
                                      emptyMessage={t(
                                        'No status code mappings configured.'
                                      )}
                                      template={{ '400': '500', '429': '503' }}
                                      valueType='string'
                                    />
                                  </FormControl>
                                  <FormMessage />
                                </FormItem>
                              )}
                            />

                            {sensitiveLocked && (
                              <p className='text-muted-foreground text-xs'>
                                {t('No permission to perform this action')}
                              </p>
                            )}
                            <fieldset
                              disabled={sensitiveLocked}
                              className='space-y-4 disabled:opacity-60'
                            >
                              <FormField
                                control={form.control}
                                name='param_override'
                                render={({ field }) => (
                                  <FormItem className='space-y-3 border-t pt-4'>
                                    <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                                      <div className='space-y-1'>
                                        <FormLabel>
                                          {t('Parameter Override')}
                                        </FormLabel>
                                        <FormDescription>
                                          {t(
                                            'Override request parameters. Cannot override stream parameter.'
                                          )}
                                        </FormDescription>
                                      </div>
                                      <div className='flex flex-wrap gap-2'>
                                        <Button
                                          type='button'
                                          variant='outline'
                                          size='sm'
                                          onClick={() =>
                                            setParamOverrideEditorOpen(true)
                                          }
                                        >
                                          <Wand2 className='mr-2 h-4 w-4' />
                                          {t('Visual edit')}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='outline'
                                          size='sm'
                                          onClick={() => {
                                            field.onChange(
                                              JSON.stringify(
                                                {
                                                  operations: [
                                                    {
                                                      path: 'temperature',
                                                      mode: 'set',
                                                      value: 0.7,
                                                      conditions: [
                                                        {
                                                          path: 'model',
                                                          mode: 'prefix',
                                                          value: 'gpt',
                                                        },
                                                      ],
                                                      logic: 'AND',
                                                    },
                                                  ],
                                                },
                                                null,
                                                2
                                              )
                                            )
                                          }}
                                        >
                                          <Code className='mr-2 h-4 w-4' />
                                          {t('New Format Template')}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          onClick={() => field.onChange('')}
                                        >
                                          {t('Clear')}
                                        </Button>
                                      </div>
                                    </div>
                                    <FormControl>
                                      <JsonCodeEditor
                                        value={field.value || ''}
                                        onChange={field.onChange}
                                        name={field.name}
                                        onBlur={field.onBlur}
                                        textareaRef={field.ref}
                                        disabled={
                                          sensitiveLocked || isSubmitting
                                        }
                                        placeholder={t(
                                          'Override request parameters. Cannot override stream parameter.'
                                        )}
                                      />
                                    </FormControl>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='header_override'
                                render={({ field }) => (
                                  <FormItem className='space-y-3 border-t pt-4'>
                                    <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                                      <div className='space-y-1'>
                                        <FormLabel>
                                          {t('Request Header Override')}
                                        </FormLabel>
                                        <FormDescription>
                                          {t('Override request headers')}
                                        </FormDescription>
                                      </div>
                                      <div className='flex flex-wrap gap-2'>
                                        <Button
                                          type='button'
                                          variant='outline'
                                          size='sm'
                                          onClick={() =>
                                            field.onChange(
                                              JSON.stringify(
                                                {
                                                  '*': true,
                                                  're:^X-Trace-.*$': true,
                                                  'X-Foo':
                                                    '{client_header:X-Foo}',
                                                  Authorization:
                                                    'Bearer {api_key}',
                                                },
                                                null,
                                                2
                                              )
                                            )
                                          }
                                        >
                                          {t('Fill Template')}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='outline'
                                          size='sm'
                                          onClick={() =>
                                            field.onChange(
                                              JSON.stringify(
                                                { '*': true },
                                                null,
                                                2
                                              )
                                            )
                                          }
                                        >
                                          {t('Passthrough Template')}
                                        </Button>
                                        <Button
                                          type='button'
                                          variant='ghost'
                                          size='sm'
                                          onClick={() => field.onChange('')}
                                        >
                                          {t('Clear')}
                                        </Button>
                                      </div>
                                    </div>
                                    <FormControl>
                                      <JsonCodeEditor
                                        value={field.value || ''}
                                        onChange={field.onChange}
                                        name={field.name}
                                        onBlur={field.onBlur}
                                        textareaRef={field.ref}
                                        disabled={
                                          sensitiveLocked || isSubmitting
                                        }
                                        placeholder={t(
                                          'Enter JSON to override request headers'
                                        )}
                                        heightClassName='h-40 min-h-40 max-h-40'
                                      />
                                    </FormControl>
                                    <FormDescription className='text-xs'>
                                      {t('Supported variables')}:{' '}
                                      <code className='bg-muted rounded px-1 py-0.5'>
                                        {'{api_key}'}
                                      </code>{' '}
                                      — {t('Channel key')},{' '}
                                      <code className='bg-muted rounded px-1 py-0.5'>
                                        {'{client_header:NAME}'}
                                      </code>{' '}
                                      — {t('Client header value')}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />
                            </fieldset>
                          </div>
                          {FIELD_PASSTHROUGH_TYPES.has(currentType) && (
                            <div
                              id={
                                ADVANCED_SETTINGS_SECTION_IDS.fieldPassthrough
                              }
                              className={sideDrawerSectionClassName(
                                configuredAdvancedSectionClassName(
                                  'scroll-mt-4',
                                  fieldPassthroughConfigured
                                )
                              )}
                            >
                              <CardHeading
                                title={t('Field passthrough controls')}
                                icon={<SlidersHorizontal className='h-4 w-4' />}
                                iconTone='chart-4'
                              />
                              <fieldset
                                disabled={sensitiveLocked}
                                className='disabled:opacity-60'
                              >
                                <div className='divide-border space-y-0 divide-y border-y'>
                                  <FormField
                                    control={form.control}
                                    name='allow_service_tier'
                                    render={({ field }) => (
                                      <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                        <div className='space-y-0.5'>
                                          <FormLabel className='text-sm'>
                                            {t(
                                              'Allow service_tier passthrough'
                                            )}
                                          </FormLabel>
                                          <FormDescription>
                                            {t(
                                              'Pass through the service_tier field'
                                            )}
                                          </FormDescription>
                                        </div>
                                        <FormControl>
                                          <Switch
                                            checked={field.value}
                                            onCheckedChange={field.onChange}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />

                                  {OPENAI_FIELD_PASSTHROUGH_TYPES.has(
                                    currentType
                                  ) && (
                                    <>
                                      <FormField
                                        control={form.control}
                                        name='disable_store'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t('Disable store passthrough')}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'When enabled, the store field will be blocked'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />

                                      <FormField
                                        control={form.control}
                                        name='allow_safety_identifier'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t(
                                                  'Allow safety_identifier passthrough'
                                                )}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'Pass through the safety_identifier field'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />

                                      <FormField
                                        control={form.control}
                                        name='allow_include_obfuscation'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t(
                                                  'Allow include usage obfuscation passthrough'
                                                )}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'Pass through the include field for usage obfuscation'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />

                                      <FormField
                                        control={form.control}
                                        name='allow_inference_geo'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t(
                                                  'Allow inference geography passthrough'
                                                )}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'Pass through the inference_geo field for geographic routing'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />
                                    </>
                                  )}

                                  {CLAUDE_FIELD_PASSTHROUGH_TYPES.has(
                                    currentType
                                  ) && (
                                    <>
                                      <FormField
                                        control={form.control}
                                        name='allow_inference_geo'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t(
                                                  'Allow inference_geo passthrough'
                                                )}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'Pass through the inference_geo field for Claude data residency region control'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />

                                      <FormField
                                        control={form.control}
                                        name='allow_speed'
                                        render={({ field }) => (
                                          <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                            <div className='space-y-0.5'>
                                              <FormLabel className='text-sm'>
                                                {t('Allow speed passthrough')}
                                              </FormLabel>
                                              <FormDescription>
                                                {t(
                                                  'Pass through the speed field for Claude inference speed mode control'
                                                )}
                                              </FormDescription>
                                            </div>
                                            <FormControl>
                                              <Switch
                                                checked={field.value}
                                                onCheckedChange={field.onChange}
                                              />
                                            </FormControl>
                                          </FormItem>
                                        )}
                                      />

                                      {currentType === 14 && (
                                        <FormField
                                          control={form.control}
                                          name='claude_beta_query'
                                          render={({ field }) => (
                                            <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                                              <div className='space-y-0.5'>
                                                <FormLabel className='text-sm'>
                                                  {t(
                                                    'Allow Claude beta query passthrough'
                                                  )}
                                                </FormLabel>
                                                <FormDescription>
                                                  {t(
                                                    'Pass through the anthropic-beta header for beta features'
                                                  )}
                                                </FormDescription>
                                              </div>
                                              <FormControl>
                                                <Switch
                                                  checked={field.value}
                                                  onCheckedChange={
                                                    field.onChange
                                                  }
                                                />
                                              </FormControl>
                                            </FormItem>
                                          )}
                                        />
                                      )}
                                    </>
                                  )}
                                </div>
                              </fieldset>
                            </div>
                          )}
                        </>
                      }
                      other={
                        <>
                          <div
                            id={ADVANCED_SETTINGS_SECTION_IDS.internalNotes}
                            className={configuredAdvancedSectionClassName(
                              'flex scroll-mt-4 flex-col gap-4 border-t pt-4',
                              internalNotesConfigured
                            )}
                          >
                            <SubHeading
                              title={t('Internal Notes')}
                              icon={<FileText className='h-3.5 w-3.5' />}
                              iconTone='chart-3'
                            />
                            <div className='grid gap-4 sm:grid-cols-2'>
                              <FormField
                                control={form.control}
                                name='tag'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('Tag')}</FormLabel>
                                    <FormControl>
                                      <Input
                                        placeholder={t(FIELD_PLACEHOLDERS.TAG)}
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(FIELD_DESCRIPTIONS.TAG)}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='remark'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('Remark')}</FormLabel>
                                    <FormControl>
                                      <Textarea
                                        placeholder={t(
                                          FIELD_PLACEHOLDERS.REMARK
                                        )}
                                        rows={2}
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(FIELD_DESCRIPTIONS.REMARK)}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />
                            </div>
                          </div>
                          <div
                            id={ADVANCED_SETTINGS_SECTION_IDS.extraSettings}
                            className={sideDrawerSectionClassName(
                              configuredAdvancedSectionClassName(
                                'scroll-mt-4',
                                extraSettingsConfigured
                              )
                            )}
                          >
                            <CardHeading
                              title={t('Channel Extra Settings')}
                              icon={<Settings className='h-4 w-4' />}
                              iconTone='chart-3'
                            />
                            {sensitiveLocked && (
                              <Alert className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-50'>
                                <AlertDescription>
                                  {t('No permission to perform this action')}
                                </AlertDescription>
                              </Alert>
                            )}
                            <fieldset
                              disabled={sensitiveLocked}
                              className='space-y-4 disabled:opacity-60'
                            >
                              <div className='divide-border space-y-0 divide-y border-y'>
                                {currentType === 1 && (
                                  <FormField
                                    control={form.control}
                                    name='force_format'
                                    render={({ field }) => (
                                      <FormItem className='flex items-center justify-between px-4 py-3'>
                                        <div className='space-y-0.5'>
                                          <FormLabel>
                                            {t('Force Format')}
                                          </FormLabel>
                                          <FormDescription>
                                            {t(
                                              'Force format response to OpenAI standard (OpenAI channel only)'
                                            )}
                                          </FormDescription>
                                        </div>
                                        <FormControl>
                                          <Switch
                                            checked={field.value}
                                            onCheckedChange={field.onChange}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />
                                )}

                                <FormField
                                  control={form.control}
                                  name='thinking_to_content'
                                  render={({ field }) => (
                                    <FormItem className='flex items-center justify-between px-4 py-3'>
                                      <div className='space-y-0.5'>
                                        <FormLabel>
                                          {t('Thinking to Content')}
                                        </FormLabel>
                                        <FormDescription>
                                          {t(
                                            'Convert reasoning_content to <think> tag in content'
                                          )}
                                        </FormDescription>
                                      </div>
                                      <FormControl>
                                        <Switch
                                          checked={field.value}
                                          onCheckedChange={field.onChange}
                                        />
                                      </FormControl>
                                    </FormItem>
                                  )}
                                />

                                <FormField
                                  control={form.control}
                                  name='pass_through_body_enabled'
                                  render={({ field }) => (
                                    <FormItem className='flex items-center justify-between px-4 py-3'>
                                      <div className='space-y-0.5'>
                                        <FormLabel>
                                          {t('Pass Through Body')}
                                        </FormLabel>
                                        <FormDescription>
                                          {t(
                                            'Pass request body directly to upstream'
                                          )}
                                        </FormDescription>
                                      </div>
                                      <FormControl>
                                        <Switch
                                          checked={field.value}
                                          onCheckedChange={field.onChange}
                                        />
                                      </FormControl>
                                    </FormItem>
                                  )}
                                />

                                <FormField
                                  control={form.control}
                                  name='disable_task_polling_sleep'
                                  render={({ field }) => (
                                    <FormItem className='flex items-center justify-between px-4 py-3'>
                                      <div className='space-y-0.5'>
                                        <FormLabel>
                                          {t('Skip async task polling delay')}
                                        </FormLabel>
                                        <FormDescription>
                                          {t(
                                            'Do not wait one second between polling async tasks for this channel'
                                          )}
                                        </FormDescription>
                                      </div>
                                      <FormControl>
                                        <Switch
                                          checked={field.value}
                                          onCheckedChange={field.onChange}
                                        />
                                      </FormControl>
                                    </FormItem>
                                  )}
                                />
                              </div>

                              <FormField
                                control={form.control}
                                name='proxy'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('Proxy Address')}</FormLabel>
                                    <FormControl>
                                      <Input
                                        placeholder={t(
                                          'socks5://user:pass@host:port'
                                        )}
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(
                                        'Network proxy for this channel (supports HTTP, HTTPS, SOCKS5, and SOCKS5H)'
                                      )}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='http_protocol'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('HTTP Protocol')}</FormLabel>
                                    <Select
                                      items={[
                                        {
                                          value: 'auto',
                                          label: t('Auto'),
                                        },
                                        {
                                          value: 'http1',
                                          label: t('HTTP/1.1'),
                                        },
                                      ]}
                                      value={field.value || 'auto'}
                                      onValueChange={(value) => {
                                        const nextProtocol =
                                          value === 'http1' ? 'http1' : 'auto'
                                        field.onChange(nextProtocol)
                                        if (nextProtocol === 'http1') {
                                          form.setValue(
                                            'http2_connection_shards',
                                            1,
                                            {
                                              shouldDirty: true,
                                              shouldValidate: true,
                                            }
                                          )
                                        }
                                      }}
                                    >
                                      <FormControl>
                                        <SelectTrigger>
                                          <SelectValue />
                                        </SelectTrigger>
                                      </FormControl>
                                      <SelectContent
                                        alignItemWithTrigger={false}
                                      >
                                        <SelectGroup>
                                          <SelectItem value='auto'>
                                            {t('Auto')}
                                          </SelectItem>
                                          <SelectItem value='http1'>
                                            {t('HTTP/1.1')}
                                          </SelectItem>
                                        </SelectGroup>
                                      </SelectContent>
                                    </Select>
                                    <FormDescription>
                                      {t(
                                        'Auto negotiates HTTP/2 when available. HTTP/1.1 forces multiple keep-alive connections under concurrency.'
                                      )}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='http2_connection_shards'
                                render={({ field }) => {
                                  const http1Selected =
                                    currentHttpProtocol === 'http1'
                                  const shardItems = Array.from(
                                    { length: 8 },
                                    (_, index) => {
                                      const value = String(index + 1)
                                      return { value, label: value }
                                    }
                                  )
                                  return (
                                    <FormItem>
                                      <FormLabel>
                                        {t('HTTP/2 Connection Shards')}
                                      </FormLabel>
                                      <Select
                                        items={shardItems}
                                        value={String(field.value || 1)}
                                        disabled={http1Selected}
                                        onValueChange={(value) => {
                                          field.onChange(Number(value))
                                        }}
                                      >
                                        <FormControl>
                                          <SelectTrigger
                                            disabled={http1Selected}
                                          >
                                            <SelectValue />
                                          </SelectTrigger>
                                        </FormControl>
                                        <SelectContent
                                          alignItemWithTrigger={false}
                                        >
                                          <SelectGroup>
                                            {shardItems.map((item) => (
                                              <SelectItem
                                                key={item.value}
                                                value={item.value}
                                              >
                                                {item.label}
                                              </SelectItem>
                                            ))}
                                          </SelectGroup>
                                        </SelectContent>
                                      </Select>
                                      <FormDescription>
                                        {http1Selected
                                          ? t(
                                              'HTTP/2 connection shards are unavailable when HTTP/1.1 is selected.'
                                            )
                                          : t(
                                              'Spread HTTP/2 traffic across multiple reusable connections to the same upstream origin (1-8).'
                                            )}
                                      </FormDescription>
                                      <FormMessage />
                                    </FormItem>
                                  )
                                }}
                              />

                              <FormField
                                control={form.control}
                                name='system_prompt'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>{t('System Prompt')}</FormLabel>
                                    <FormControl>
                                      <Textarea
                                        placeholder={t(
                                          'Enter system prompt (user prompt takes priority)'
                                        )}
                                        rows={3}
                                        {...field}
                                      />
                                    </FormControl>
                                    <FormDescription>
                                      {t(
                                        'Default system prompt for this channel'
                                      )}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />

                              <FormField
                                control={form.control}
                                name='system_prompt_override'
                                render={({ field }) => (
                                  <FormItem className='flex items-center justify-between'>
                                    <div className='space-y-0.5'>
                                      <FormLabel>
                                        {t('System Prompt Concatenation')}
                                      </FormLabel>
                                      <FormDescription>
                                        {t(
                                          'Concatenate channel system prompt with user&apos;s prompt'
                                        )}
                                      </FormDescription>
                                    </div>
                                    <FormControl>
                                      <Switch
                                        checked={field.value}
                                        onCheckedChange={field.onChange}
                                      />
                                    </FormControl>
                                  </FormItem>
                                )}
                              />
                            </fieldset>
                          </div>
                          <div
                            id={ADVANCED_SETTINGS_SECTION_IDS.imageOutput}
                            className={sideDrawerSectionClassName(
                              configuredAdvancedSectionClassName(
                                'scroll-mt-4',
                                imageOutputConfigured
                              )
                            )}
                          >
                            <CardHeading
                              title={t('Output strategy')}
                              icon={<Sparkles className='h-4 w-4' />}
                              iconTone='chart-3'
                            />
                            <fieldset
                              disabled={sensitiveLocked}
                              className='disabled:opacity-60'
                            >
                              <FormField
                                control={form.control}
                                name='image_output_strategy'
                                render={({ field }) => (
                                  <FormItem>
                                    <FormLabel>
                                      {t('Generated media output')}
                                    </FormLabel>
                                    <Select
                                      items={[
                                        {
                                          value: 'oss',
                                          label: t('Aliyun OSS URL'),
                                        },
                                        {
                                          value: 'r2',
                                          label: t('Cloudflare R2 URL'),
                                        },
                                        {
                                          value: 'local_temp_cf',
                                          label: t(
                                            'Local temporary URL via Cloudflare (24 hours)'
                                          ),
                                        },
                                        {
                                          value: 'local_temp_esa',
                                          label: t(
                                            'Local temporary URL via ESA (24 hours)'
                                          ),
                                        },
                                        {
                                          value: 'passthrough',
                                          label: t('Upstream passthrough'),
                                        },
                                      ]}
                                      value={field.value ?? 'passthrough'}
                                      onValueChange={field.onChange}
                                    >
                                      <FormControl>
                                        <SelectTrigger>
                                          <SelectValue
                                            placeholder={t(
                                              'Keep current output behavior'
                                            )}
                                          />
                                        </SelectTrigger>
                                      </FormControl>
                                      <SelectContent
                                        alignItemWithTrigger={false}
                                      >
                                        <SelectGroup>
                                          <SelectItem value='oss'>
                                            {t('Aliyun OSS URL')}
                                          </SelectItem>
                                          <SelectItem value='r2'>
                                            {t('Cloudflare R2 URL')}
                                          </SelectItem>
                                          <SelectItem value='local_temp_cf'>
                                            {t(
                                              'Local temporary URL via Cloudflare (24 hours)'
                                            )}
                                          </SelectItem>
                                          <SelectItem value='local_temp_esa'>
                                            {t(
                                              'Local temporary URL via ESA (24 hours)'
                                            )}
                                          </SelectItem>
                                          <SelectItem value='passthrough'>
                                            {t('Upstream passthrough')}
                                          </SelectItem>
                                        </SelectGroup>
                                      </SelectContent>
                                    </Select>
                                    <FormDescription>
                                      {t(
                                        'Generated images and videos use upstream passthrough by default. Aliyun OSS stores durable output under output/. Cloudflare R2 also supports durable media output; local temporary options apply to images only.'
                                      )}
                                    </FormDescription>
                                    <FormMessage />
                                  </FormItem>
                                )}
                              />
                              {GEMINI_FILE_DATA_CHANNEL_TYPES.has(
                                currentType
                              ) && (
                                <>
                                  <Separator className='my-4' />
                                  <FormField
                                    control={form.control}
                                    name='gemini_file_data_enabled'
                                    render={({ field }) => (
                                      <FormItem className='flex items-center justify-between gap-3'>
                                        <div className='space-y-0.5'>
                                          <FormLabel>
                                            {t(
                                              'Use Gemini fileData for input images'
                                            )}
                                          </FormLabel>
                                          <FormDescription>
                                            {t(
                                              'Let this upstream fetch public image URLs instead of uploading inline Base64 data. Enable only after verifying upstream support.'
                                            )}
                                          </FormDescription>
                                        </div>
                                        <FormControl>
                                          <Switch
                                            checked={field.value}
                                            onCheckedChange={field.onChange}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />
                                </>
                              )}
                            </fieldset>
                          </div>
                          {MODEL_FETCHABLE_TYPES.has(currentType) && (
                            <div
                              id={
                                ADVANCED_SETTINGS_SECTION_IDS.upstreamModelDetection
                              }
                              className={sideDrawerSectionClassName(
                                configuredAdvancedSectionClassName(
                                  'scroll-mt-4',
                                  upstreamModelDetectionConfigured
                                )
                              )}
                            >
                              <CardHeading
                                title={t('Upstream Model Detection Settings')}
                                icon={<RefreshCw className='h-4 w-4' />}
                                iconTone='info'
                              />
                              <fieldset
                                disabled={sensitiveLocked}
                                className='space-y-4 disabled:opacity-60'
                              >
                                <div className='divide-border space-y-0 divide-y border-y'>
                                  <FormField
                                    control={form.control}
                                    name='upstream_model_update_check_enabled'
                                    render={({ field }) => (
                                      <FormItem className='flex items-center justify-between px-4 py-3'>
                                        <div className='space-y-0.5'>
                                          <FormLabel>
                                            {t('Upstream Model Update Check')}
                                          </FormLabel>
                                          <FormDescription>
                                            {t(
                                              'Periodically check for upstream model changes'
                                            )}
                                          </FormDescription>
                                        </div>
                                        <FormControl>
                                          <Switch
                                            checked={field.value}
                                            onCheckedChange={field.onChange}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />
                                  <FormField
                                    control={form.control}
                                    name='upstream_model_update_auto_sync_enabled'
                                    render={({ field }) => (
                                      <FormItem className='flex items-center justify-between px-4 py-3'>
                                        <div className='space-y-0.5'>
                                          <FormLabel>
                                            {t('Auto Sync Upstream Models')}
                                          </FormLabel>
                                          <FormDescription>
                                            {t(
                                              'Automatically sync model list when upstream changes are detected'
                                            )}
                                          </FormDescription>
                                        </div>
                                        <FormControl>
                                          <Switch
                                            checked={field.value}
                                            disabled={
                                              !upstreamModelUpdateCheckEnabled
                                            }
                                            onCheckedChange={field.onChange}
                                          />
                                        </FormControl>
                                      </FormItem>
                                    )}
                                  />
                                </div>
                                <FormField
                                  control={form.control}
                                  name='upstream_model_update_ignored_models'
                                  render={({ field }) => (
                                    <FormItem>
                                      <FormLabel>
                                        {t('Ignored upstream models')}
                                      </FormLabel>
                                      <FormControl>
                                        <Input
                                          placeholder={t(
                                            'e.g., gpt-4.1-nano,regex:^claude-.*$,regex:^sora-.*$'
                                          )}
                                          {...field}
                                        />
                                      </FormControl>
                                      <FormDescription>
                                        {t(
                                          'Comma-separated exact model names. Prefix with regex: to ignore by regular expression.'
                                        )}
                                      </FormDescription>
                                      <FormMessage />
                                    </FormItem>
                                  )}
                                />
                                <div className='text-muted-foreground space-y-2 border-t pt-3 text-xs'>
                                  <div>
                                    <span className='text-foreground font-medium'>
                                      {t('Last check time')}:
                                    </span>{' '}
                                    {formatUnixTime(
                                      upstreamUpdateMeta.lastCheckTime
                                    )}
                                  </div>
                                  <div>
                                    <span className='text-foreground font-medium'>
                                      {t('Last detected addable models')}:
                                    </span>{' '}
                                    {upstreamUpdateMeta.detectedModels
                                      .length === 0 ? (
                                      t('None')
                                    ) : (
                                      <>
                                        <span className='break-all'>
                                          {upstreamDetectedModelsPreview.join(
                                            ', '
                                          )}
                                        </span>
                                        {upstreamDetectedModelsOmittedCount >
                                          0 && (
                                          <span className='ml-1'>
                                            {t(
                                              '({{total}} total, {{omit}} omitted)',
                                              {
                                                total:
                                                  upstreamUpdateMeta
                                                    .detectedModels.length,
                                                omit: upstreamDetectedModelsOmittedCount,
                                              }
                                            )}
                                          </span>
                                        )}
                                      </>
                                    )}
                                  </div>
                                </div>
                              </fieldset>
                            </div>
                          )}
                        </>
                      }
                    />
                  </div>
                </>
              )}
            </form>
          </Form>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose
              render={<Button variant='outline' disabled={isSubmitting} />}
            >
              {t('Cancel')}
            </SheetClose>
            <>
              {providerPickerOpen && providerChosen && (
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => setProviderPickerOpen(false)}
                >
                  {t('Back')}
                </Button>
              )}
              {!providerPickerOpen && (
                <Button
                  form='channel-form'
                  type='submit'
                  disabled={isSubmitting || isChannelDetailUnavailable}
                >
                  {isSubmitting && (
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  )}
                  {isEditing ? t('Update Channel') : t('Save changes')}
                </Button>
              )}
            </>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {paramOverrideEditorOpen && !sensitiveLocked && (
        <ParamOverrideEditorDialog
          open={paramOverrideEditorOpen}
          value={form.watch('param_override') || ''}
          onOpenChange={setParamOverrideEditorOpen}
          onSave={(nextValue) => {
            form.setValue('param_override', nextValue, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }}
        />
      )}

      {advancedCustomEditorOpen && !sensitiveLocked && (
        <AdvancedCustomEditorDialog
          open={advancedCustomEditorOpen}
          value={form.watch('advanced_custom') || ''}
          onOpenChange={setAdvancedCustomEditorOpen}
          onSave={(nextValue) => {
            form.setValue('advanced_custom', nextValue, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }}
        />
      )}

      <SecureVerificationDialog {...verification.dialogProps} />

      {/* Missing Models Confirmation Dialog */}
      <MissingModelsConfirmationDialog
        open={missingModelsDialogOpen}
        missingModels={missingModelsList}
        onConfirm={handleMissingModelsAction}
        onOpenChange={setMissingModelsDialogOpen}
      />

      <StatusCodeRiskDialog
        open={statusCodeRiskOpen}
        onOpenChange={(v) => {
          if (!v) handleStatusCodeRiskAction(false)
        }}
        detailItems={statusCodeRiskDetailItems}
        onConfirm={() => handleStatusCodeRiskAction(true)}
      />
    </>
  )
}
