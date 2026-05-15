/*
Copyright (C) 2025 QuantumNous

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

import React, { useEffect, useState, useRef } from 'react';
import {
  API,
  showError,
  renderGroupOption,
} from '../../../../helpers';
import {
  Button,
  Modal,
  Space,
  Tag,
  Typography,
  Select,
  Switch,
} from '@douyinfe/semi-ui';
import {
  IconSave,
  IconClose,
  IconChevronUp,
  IconChevronDown,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const BatchEditGroupModal = (props) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [selectedGroups, setSelectedGroups] = useState([]);
  const [crossGroupRetry, setCrossGroupRetry] = useState(false);
  const [dragIndex, setDragIndex] = useState(null);
  const groupsLoadedRef = useRef(false);

  useEffect(() => {
    if (props.visible && !groupsLoadedRef.current) {
      loadGroups();
    }
  }, [props.visible]);

  // Reset state on open
  useEffect(() => {
    if (props.visible) {
      setSelectedGroups([]);
      setCrossGroupRetry(false);
    }
  }, [props.visible]);

  const loadGroups = async () => {
    let res = await API.get(`/api/user/self/groups`);
    const { success, message, data } = res.data;
    if (success) {
      let localGroupOptions = Object.entries(data).map(([group, info]) => ({
        label: info.desc,
        value: group,
        ratio: info.ratio,
      }));
      if (!localGroupOptions.some((g) => g.value === 'auto')) {
        localGroupOptions.unshift({
          label: t('自动分组（系统默认排序）'),
          value: 'auto',
          ratio: undefined,
        });
      }
      setGroups(localGroupOptions);
      groupsLoadedRef.current = true;
    } else {
      showError(t(message));
    }
  };

  // 多选下拉变更：auto 和具体分组互斥
  const handleGroupSelectChange = (newValues) => {
    if (!newValues || newValues.length === 0) {
      setSelectedGroups([]);
      return;
    }
    const prevHadAuto = selectedGroups.includes('auto');
    const nowHasAuto = newValues.includes('auto');

    if (nowHasAuto && !prevHadAuto) {
      setSelectedGroups(['auto']);
      return;
    }
    if (nowHasAuto && prevHadAuto && newValues.length > 1) {
      setSelectedGroups(newValues.filter((g) => g !== 'auto'));
      return;
    }
    const newSet = new Set(newValues);
    const ordered = selectedGroups.filter((g) => g !== 'auto' && newSet.has(g));
    newValues.forEach((g) => {
      if (g !== 'auto' && !ordered.includes(g)) {
        ordered.push(g);
      }
    });
    setSelectedGroups(ordered);
  };

  const moveGroupUp = (index) => {
    if (index <= 0) return;
    setSelectedGroups((prev) => {
      const next = [...prev];
      [next[index - 1], next[index]] = [next[index], next[index - 1]];
      return next;
    });
  };

  const moveGroupDown = (index) => {
    setSelectedGroups((prev) => {
      if (index >= prev.length - 1) return prev;
      const next = [...prev];
      [next[index], next[index + 1]] = [next[index + 1], next[index]];
      return next;
    });
  };

  const resetSelectedGroups = () => {
    setSelectedGroups(['auto']);
  };

  const handleDragStart = (e, index) => {
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', index.toString());
    setDragIndex(index);
  };

  const handleDragOver = (e, index) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    if (dragIndex !== null && dragIndex !== index) {
      setSelectedGroups((prev) => {
        const next = [...prev];
        const [removed] = next.splice(dragIndex, 1);
        next.splice(index, 0, removed);
        return next;
      });
      setDragIndex(index);
    }
  };

  const handleDragEnd = () => {
    setDragIndex(null);
  };

  const getGroupLabel = (groupValue) => {
    const found = groups.find((g) => g.value === groupValue);
    if (!found) return groupValue;
    return found.label || groupValue;
  };

  const getGroupRatio = (groupValue) => {
    const found = groups.find((g) => g.value === groupValue);
    return found?.ratio;
  };

  const deriveGroupFields = () => {
    if (selectedGroups.length === 0) {
      return { group: '', auto_group_priority: '' };
    }
    if (selectedGroups.length === 1) {
      return { group: selectedGroups[0], auto_group_priority: '' };
    }
    return {
      group: 'auto',
      auto_group_priority: JSON.stringify(selectedGroups),
    };
  };

  const handleSubmit = () => {
    const derived = deriveGroupFields();
    setLoading(true);
    props.onSubmit({
      group: derived.group,
      auto_group_priority: derived.auto_group_priority,
      cross_group_retry: crossGroupRetry,
    }).finally(() => {
      setLoading(false);
    });
  };

  const isAutoMode = selectedGroups.length === 1 && selectedGroups[0] === 'auto';
  const showPriorityPanel = selectedGroups.length >= 2 && !selectedGroups.includes('auto');

  const getGroupHint = () => {
    if (selectedGroups.length === 0) return t('不选 = 继承用户默认分组');
    if (isAutoMode) return t('自动模式，使用系统默认分组排序');
    if (selectedGroups.length === 1) return t('单选 = 固定使用该分组');
    return t(`多选 = 自动模式，按优先级尝试 ${selectedGroups.length} 个分组`);
  };

  return (
    <Modal
      title={t('批量编辑分组')}
      visible={props.visible}
      onCancel={props.onCancel}
      footer={
        <Space>
          <Button
            theme='solid'
            onClick={handleSubmit}
            icon={<IconSave />}
            loading={loading}
          >
            {t('确认更新')}
          </Button>
          <Button
            theme='light'
            type='primary'
            onClick={props.onCancel}
            icon={<IconClose />}
          >
            {t('取消')}
          </Button>
        </Space>
      }
      closeIcon={null}
      width={520}
    >
      <div className='p-2'>
        {/* Selected count tip */}
        <div
          className='mb-4 p-3 rounded-lg'
          style={{ backgroundColor: 'var(--semi-color-fill-0)' }}
        >
          <Text strong>
            {t('已选择 {{count}} 个令牌', { count: props.selectedCount || 0 })}
          </Text>
          <Text type='tertiary' className='ml-2'>
            {t('将统一更新以下分组设置')}
          </Text>
        </div>

        {/* Group selection */}
        {groups.length > 0 ? (
          <div className='mb-4'>
            <div className='mb-1'>
              <Text strong className='text-sm'>{t('令牌分组')}</Text>
            </div>
            <Select
              multiple
              value={selectedGroups}
              onChange={handleGroupSelectChange}
              placeholder={t('请选择分组，多选=自动模式')}
              optionList={groups}
              renderOptionItem={renderGroupOption}
              filter={(input, option) => {
                const q = input.toLowerCase();
                return (
                  option.value?.toLowerCase().includes(q) ||
                  (typeof option.label === 'string' &&
                    option.label.toLowerCase().includes(q))
                );
              }}
              showClear
              style={{ width: '100%' }}
            />
            <div className='text-xs mt-1' style={{ color: 'var(--semi-color-text-2)' }}>
              {getGroupHint()}
            </div>
          </div>
        ) : (
          <div className='mb-4'>
            <Text type='tertiary'>{t('加载分组中...')}</Text>
          </div>
        )}

        {/* Cross-group retry */}
        <div
          className='mb-4'
          style={{ display: (isAutoMode || showPriorityPanel) ? 'block' : 'none' }}
        >
          <div className='flex items-center justify-between'>
            <Text strong className='text-sm'>{t('跨分组重试')}</Text>
            <Switch
              checked={crossGroupRetry}
              onChange={setCrossGroupRetry}
              size='default'
            />
          </div>
          <div className='text-xs mt-1' style={{ color: 'var(--semi-color-text-2)' }}>
            {t('开启后，当前分组渠道失败时会按顺序尝试下一个分组的渠道')}
          </div>
        </div>

        {/* Priority panel */}
        <div style={{ display: showPriorityPanel ? 'block' : 'none' }}>
          <div className='flex items-center justify-between mb-2 mt-4'>
            <Text strong className='text-sm'>
              {t('分组优先级')}
            </Text>
            <Button
              theme='light'
              size='small'
              onClick={resetSelectedGroups}
              icon={<IconRefresh />}
            >
              {t('恢复默认')}
            </Button>
          </div>
          <Text type='tertiary' size='small' className='block mb-2'>
            {t('拖拽或点击按钮调整尝试顺序，排在上方的分组优先被使用')}
          </Text>
          <div
            style={{
              border: '1px solid var(--semi-color-border)',
              borderRadius: '8px',
              overflow: 'hidden',
            }}
          >
            {selectedGroups.map((groupValue, index) => (
              <div
                key={groupValue}
                draggable
                onDragStart={(e) => handleDragStart(e, index)}
                onDragOver={(e) => handleDragOver(e, index)}
                onDragEnd={handleDragEnd}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '8px 12px',
                  cursor: 'grab',
                  borderBottom:
                    index < selectedGroups.length - 1
                      ? '1px solid var(--semi-color-border)'
                      : 'none',
                  backgroundColor:
                    dragIndex === index
                      ? 'var(--semi-color-primary-light-default)'
                      : index % 2 === 0
                        ? 'var(--semi-color-fill-0)'
                        : 'transparent',
                  opacity: dragIndex === index ? 0.5 : 1,
                }}
              >
                <div className='flex items-center gap-2'>
                  <span
                    style={{
                      color: 'var(--semi-color-text-2)',
                      cursor: 'grab',
                      fontSize: '16px',
                      userSelect: 'none',
                    }}
                  >
                    ⠿
                  </span>
                  <Tag size='small' color={index === 0 ? 'blue' : 'grey'}>
                    #{index + 1}
                  </Tag>
                  <Text>{getGroupLabel(groupValue)}</Text>
                  <Text type='tertiary' size='small'>
                    {getGroupRatio(groupValue) !== undefined
                      ? `${getGroupRatio(groupValue)}x`
                      : ''}
                  </Text>
                </div>
                <Space spacing={4}>
                  <Button
                    theme='light'
                    size='small'
                    disabled={index === 0}
                    onClick={() => moveGroupUp(index)}
                    icon={<IconChevronUp />}
                  />
                  <Button
                    theme='light'
                    size='small'
                    disabled={index === selectedGroups.length - 1}
                    onClick={() => moveGroupDown(index)}
                    icon={<IconChevronDown />}
                  />
                </Space>
              </div>
            ))}
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default BatchEditGroupModal;
