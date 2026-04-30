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

import React from 'react';
import { Layout } from '@douyinfe/semi-ui';
import CardPro from '../../components/common/ui/CardPro';
import TaskLogsTable from '../../components/table/task-logs/TaskLogsTable';
import TaskLogsActions from '../../components/table/task-logs/TaskLogsActions';
import TaskLogsFilters from '../../components/table/task-logs/TaskLogsFilters';
import ColumnSelectorModal from '../../components/table/task-logs/modals/ColumnSelectorModal';
import ContentModal from '../../components/table/task-logs/modals/ContentModal';
import AudioPreviewModal from '../../components/table/task-logs/modals/AudioPreviewModal';
import { useAsyncImageLogsData } from '../../hooks/async-image-logs/useAsyncImageLogsData';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../helpers/utils';

const AsyncImage = () => {
  const asyncImageLogsData = useAsyncImageLogsData();
  const isMobile = useIsMobile();

  return (
    <div className='mt-[60px] px-2'>
      <ColumnSelectorModal {...asyncImageLogsData} />
      <ContentModal {...asyncImageLogsData} isVideo={false} />
      <ContentModal
        isModalOpen={asyncImageLogsData.isVideoModalOpen}
        setIsModalOpen={asyncImageLogsData.setIsVideoModalOpen}
        modalContent={asyncImageLogsData.videoUrl}
        isVideo={true}
      />
      <AudioPreviewModal
        isModalOpen={asyncImageLogsData.isAudioModalOpen}
        setIsModalOpen={asyncImageLogsData.setIsAudioModalOpen}
        audioClips={asyncImageLogsData.audioClips}
      />

      <Layout>
        <CardPro
          type='type2'
          statsArea={<TaskLogsActions {...asyncImageLogsData} title={asyncImageLogsData.t('异步绘图记录')} />}
          searchArea={<TaskLogsFilters {...asyncImageLogsData} />}
          paginationArea={createCardProPagination({
            currentPage: asyncImageLogsData.activePage,
            pageSize: asyncImageLogsData.pageSize,
            total: asyncImageLogsData.logCount,
            onPageChange: asyncImageLogsData.handlePageChange,
            onPageSizeChange: asyncImageLogsData.handlePageSizeChange,
            isMobile: isMobile,
            t: asyncImageLogsData.t,
          })}
          t={asyncImageLogsData.t}
        >
          <TaskLogsTable
            {...asyncImageLogsData}
            getDurationColor={(durationSec) => {
              if (durationSec > 300) return 'red';
              if (durationSec > 100) return 'yellow';
              return 'green';
            }}
          />
        </CardPro>
      </Layout>
    </div>
  );
};

export default AsyncImage;
