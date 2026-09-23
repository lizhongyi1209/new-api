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
export type ChannelConfigurationSection =
  | 'connection'
  | 'routing'
  | 'request'
  | 'other'
export type ChannelConfigurationStatus =
  | 'idle'
  | 'ready'
  | 'configured'
  | 'error'

const FIELD_SECTIONS: Record<string, ChannelConfigurationSection> = {
  model_mapping: 'routing',
  priority: 'routing',
  weight: 'routing',
  test_model: 'routing',
  auto_ban: 'routing',
  status_code_mapping: 'request',
  param_override: 'request',
  header_override: 'request',
  allow_service_tier: 'request',
  disable_store: 'request',
  allow_safety_identifier: 'request',
  allow_include_obfuscation: 'request',
  allow_inference_geo: 'request',
  allow_speed: 'request',
  claude_beta_query: 'request',
  tag: 'other',
  remark: 'other',
  setting: 'other',
  settings: 'other',
  force_format: 'other',
  thinking_to_content: 'other',
  pass_through_body_enabled: 'other',
  disable_task_polling_sleep: 'other',
  proxy: 'other',
  http_protocol: 'other',
  http2_connection_shards: 'other',
  system_prompt: 'other',
  system_prompt_override: 'other',
  image_output_strategy: 'other',
  gemini_file_data_enabled: 'other',
  upstream_model_update_check_enabled: 'other',
  upstream_model_update_auto_sync_enabled: 'other',
  upstream_model_update_ignored_models: 'other',
}

export function getChannelConfigurationSectionForField(
  field: string
): ChannelConfigurationSection {
  return FIELD_SECTIONS[field] ?? 'connection'
}
