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
import { Button, Modal, Space, Typography } from '@douyinfe/semi-ui';

const COLOR_OPTIONS = [
  { label: '红色', value: '#ef4444' },
  { label: '橙色', value: '#f97316' },
  { label: '黄色', value: '#eab308' },
  { label: '绿色', value: '#22c55e' },
  { label: '青色', value: '#06b6d4' },
  { label: '蓝色', value: '#3b82f6' },
  { label: '紫色', value: '#8b5cf6' },
  { label: '粉色', value: '#ec4899' },
];

const getChannelColor = (settings) => {
  if (!settings) return '';
  try {
    return JSON.parse(settings)?.color || '';
  } catch {
    return '';
  }
};

const ChannelColorModal = ({
  visible,
  channel,
  loading,
  onCancel,
  onSelect,
  t,
}) => {
  const currentColor = getChannelColor(channel?.settings);

  return (
    <Modal
      title={t('设置颜色')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      width={360}
      centered
    >
      <Typography.Text type='tertiary'>{channel?.name || '-'}</Typography.Text>
      <div className='mt-4 grid grid-cols-4 gap-3'>
        {COLOR_OPTIONS.map((option) => (
          <button
            key={option.value}
            type='button'
            title={t(option.label)}
            aria-label={t(option.label)}
            disabled={loading}
            className='relative flex h-12 items-center justify-center rounded-lg border border-transparent transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-60 dark:hover:bg-gray-800'
            onClick={() => onSelect(option.value)}
          >
            <span
              className='h-7 w-7 rounded-full border border-black/10'
              style={{ backgroundColor: option.value }}
            />
            {currentColor === option.value ? (
              <span className='absolute inset-1 rounded-md border-2 border-blue-500' />
            ) : null}
          </button>
        ))}
      </div>
      {currentColor ? (
        <Space className='mt-4 w-full justify-end'>
          <Button disabled={loading} onClick={() => onSelect('')}>
            {t('清除颜色')}
          </Button>
        </Space>
      ) : null}
    </Modal>
  );
};

export default ChannelColorModal;
