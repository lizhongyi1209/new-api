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

import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Card,
  Collapse,
  Empty,
  Skeleton,
  Table,
  Typography,
} from '@douyinfe/semi-ui';

import { API, renderQuotaWithAmount, timestamp2string } from '../../helpers';

const { Text, Title } = Typography;

const Referrals = () => {
  const { t } = useTranslation();
  const [summary, setSummary] = useState(null);
  const [summaryLoading, setSummaryLoading] = useState(true);
  const [summaryError, setSummaryError] = useState(false);
  const [activeKeys, setActiveKeys] = useState([]);
  const [details, setDetails] = useState({});
  const [detailLoading, setDetailLoading] = useState({});
  const requestedLevels = useRef(new Set());

  useEffect(() => {
    const loadSummary = async () => {
      try {
        const response = await API.get('/api/user/referrals');
        if (!response.data.success) {
          throw new Error(response.data.message);
        }
        setSummary(response.data.data);
      } catch {
        setSummaryError(true);
      } finally {
        setSummaryLoading(false);
      }
    };
    loadSummary();
  }, []);

  const loadLevel = async (level) => {
    if (requestedLevels.current.has(level)) return;
    requestedLevels.current.add(level);
    setDetailLoading((current) => ({ ...current, [level]: true }));
    try {
      const response = await API.get(`/api/user/referrals/${level}`);
      if (!response.data.success) {
        throw new Error(response.data.message);
      }
      setDetails((current) => ({ ...current, [level]: response.data.data }));
    } catch {
      requestedLevels.current.delete(level);
      setDetails((current) => ({ ...current, [level]: null }));
    } finally {
      setDetailLoading((current) => ({ ...current, [level]: false }));
    }
  };

  const handleCollapseChange = (keys) => {
    const nextKeys = Array.isArray(keys) ? keys : [keys];
    setActiveKeys(nextKeys);
    nextKeys.forEach((key) => loadLevel(Number(key)));
  };

  const columns = [
    {
      title: t('用户名'),
      dataIndex: 'username',
      key: 'username',
    },
    {
      title: t('注册时间'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (value) => timestamp2string(value),
    },
    {
      title: t('总充值'),
      dataIndex: 'total_top_up',
      key: 'total_top_up',
      align: 'right',
      render: (value) => renderQuotaWithAmount(value),
    },
  ];

  return (
    <div className='w-full max-w-5xl mx-auto min-h-screen lg:min-h-0 mt-[60px] px-2 pb-6'>
      <div className='mb-4'>
        <Title heading={4}>{t('推荐计划')}</Title>
        <Text type='tertiary'>{t('推荐用户数量与充值总额')}</Text>
      </div>

      {summaryLoading ? (
        <Skeleton
          active
          placeholder={<Skeleton.Paragraph rows={8} />}
          loading
        />
      ) : summaryError || !summary ? (
        <Banner type='danger' description={t('加载推荐计划失败')} />
      ) : (
        <>
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4'>
            <Card>
              <Text type='tertiary'>{t('推荐用户总数')}</Text>
              <div className='text-2xl font-semibold mt-2 tabular-nums'>
                {summary.total_count}
              </div>
            </Card>
            <Card>
              <Text type='tertiary'>{t('总充值')}</Text>
              <div className='text-2xl font-semibold mt-2 tabular-nums'>
                {renderQuotaWithAmount(summary.total_top_up)}
              </div>
            </Card>
          </div>

          <Card bodyStyle={{ padding: 0 }}>
            <Collapse activeKey={activeKeys} onChange={handleCollapseChange}>
              {summary.levels.map((level) => (
                <Collapse.Panel
                  key={level.level}
                  itemKey={String(level.level)}
                  header={
                    <div className='flex items-center justify-between gap-4 w-full pr-3'>
                      <div>
                        <div className='font-medium'>
                          {t('第 {{level}} 层推荐用户', {
                            level: level.level,
                          })}
                        </div>
                        <Text type='tertiary' size='small'>
                          {t('{{count}} 位推荐用户', { count: level.count })}
                        </Text>
                      </div>
                      <div className='text-right shrink-0'>
                        <Text type='tertiary' size='small'>
                          {t('总充值')}
                        </Text>
                        <div className='font-semibold tabular-nums'>
                          {renderQuotaWithAmount(level.total_top_up)}
                        </div>
                      </div>
                    </div>
                  }
                >
                  {detailLoading[level.level] ? (
                    <Skeleton
                      active
                      placeholder={<Skeleton.Paragraph rows={3} />}
                      loading
                    />
                  ) : details[level.level] === null ? (
                    <Banner type='danger' description={t('加载推荐用户失败')} />
                  ) : (
                    <Table
                      columns={columns}
                      dataSource={details[level.level] || []}
                      rowKey={(record) =>
                        `${record.username}-${record.created_at}`
                      }
                      pagination={false}
                      size='small'
                      empty={
                        <Empty
                          title={t('暂无推荐用户')}
                          description={t('邀请用户注册后，详情会显示在这里')}
                        />
                      }
                    />
                  )}
                </Collapse.Panel>
              ))}
            </Collapse>
          </Card>
        </>
      )}
    </div>
  );
};

export default Referrals;
