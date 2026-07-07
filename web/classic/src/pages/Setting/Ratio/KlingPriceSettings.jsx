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
import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Input,
  InputNumber,
  Radio,
  RadioGroup,
  Table,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { IconCopy, IconDelete, IconPlus, IconSearch } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

const OPTION_KEY = 'kling.prices';
const COMMON_MODELS = ['kling-v1', 'kling-v1-6', 'kling-v2-master'];

function createRow(row = {}) {
  const hasPrice = Object.prototype.hasOwnProperty.call(row, 'price_per_second');
  return {
    id: `${Date.now()}-${Math.random()}`,
    model: row.model ?? '',
    mode: row.mode ?? 'std',
    sound: row.sound ?? 'off',
    price_per_second: hasPrice ? row.price_per_second : '',
  };
}

function normalizeText(value) {
  return String(value ?? '').trim();
}

function normalizeLower(value) {
  return normalizeText(value).toLowerCase();
}

function parseRowsFromOption(raw) {
  if (!raw) {
    return [];
  }
  const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw;
  if (!Array.isArray(parsed)) {
    return [];
  }
  return parsed.map(createRow);
}

function formatPrice(value, seconds) {
  if (value === '' || value === null || value === undefined) {
    return '-';
  }
  const price = Number(value);
  if (!Number.isFinite(price)) {
    return '-';
  }
  return `$${(price * seconds).toFixed(4)}`;
}

function validateAndNormalizeRows(rows) {
  const seen = new Set();
  const normalized = [];

  rows.forEach((row, index) => {
    const line = index + 1;
    const model = normalizeText(row.model);
    const mode = normalizeLower(row.mode);
    const sound = normalizeLower(row.sound);
    const rawPrice = row.price_per_second;
    const price = Number(row.price_per_second);

    if (!model || !mode || !sound) {
      throw new Error(`第 ${line} 行：模型名称、mode、sound 不能为空`);
    }
    if (
      rawPrice === '' ||
      rawPrice === null ||
      rawPrice === undefined ||
      !Number.isFinite(price) ||
      price < 0
    ) {
      throw new Error(`第 ${line} 行：每秒价格必须填写且大于等于 0`);
    }

    const key = `${model}|${mode}|${sound}`;
    if (seen.has(key)) {
      throw new Error(`重复组合：${key}`);
    }
    seen.add(key);

    normalized.push({
      model,
      mode,
      sound,
      price_per_second: price,
    });
  });

  return normalized.sort((a, b) => {
    const modelCompare = a.model.localeCompare(b.model);
    if (modelCompare !== 0) return modelCompare;
    const modeCompare = a.mode.localeCompare(b.mode);
    if (modeCompare !== 0) return modeCompare;
    return a.sound.localeCompare(b.sound);
  });
}

export default function KlingPriceSettings({ options, refresh }) {
  const { t } = useTranslation();
  const [rows, setRows] = useState([]);
  const [mode, setMode] = useState('visual');
  const [jsonText, setJsonText] = useState('[]');
  const [jsonError, setJsonError] = useState('');
  const [search, setSearch] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    try {
      const parsedRows = parseRowsFromOption(options?.[OPTION_KEY]);
      setRows(parsedRows);
      setJsonText(JSON.stringify(validateAndNormalizeRows(parsedRows), null, 2));
      setJsonError('');
    } catch (e) {
      setRows([]);
      setJsonText(options?.[OPTION_KEY] || '[]');
      setJsonError(e.message);
    }
  }, [options]);

  const syncToJson = (nextRows) => {
    setRows(nextRows);
    try {
      setJsonText(JSON.stringify(validateAndNormalizeRows(nextRows), null, 2));
      setJsonError('');
    } catch {
      setJsonText(JSON.stringify(nextRows.map(({ id, ...rest }) => rest), null, 2));
    }
  };

  const syncToVisual = (text) => {
    setJsonText(text);
    try {
      const parsed = JSON.parse(text);
      if (!Array.isArray(parsed)) {
        setJsonError(t('JSON 必须是数组'));
        return;
      }
      const normalized = validateAndNormalizeRows(parsed);
      setRows(normalized.map(createRow));
      setJsonError('');
    } catch (e) {
      setJsonError(e.message);
    }
  };

  const updateRow = (id, field, value) => {
    syncToJson(rows.map((row) => (row.id === id ? { ...row, [field]: value } : row)));
  };

  const addRow = (model = '') => {
    syncToJson([...rows, createRow({ model, mode: 'std', sound: 'off' })]);
  };

  const copyRow = (record) => {
    syncToJson([...rows, createRow(record)]);
  };

  const removeRow = (id) => {
    syncToJson(rows.filter((row) => row.id !== id));
  };

  const filteredRows = useMemo(() => {
    const keyword = normalizeLower(search);
    if (!keyword) {
      return rows;
    }
    return rows.filter((row) =>
      [row.model, row.mode, row.sound].some((value) => normalizeLower(value).includes(keyword))
    );
  }, [rows, search]);

  const validationError = useMemo(() => {
    try {
      validateAndNormalizeRows(rows);
      return '';
    } catch (e) {
      return e.message;
    }
  }, [rows]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const normalized = validateAndNormalizeRows(mode === 'json' ? JSON.parse(jsonText) : rows);
      const res = await API.put('/api/option/', {
        key: OPTION_KEY,
        value: JSON.stringify(normalized),
      });
      if (res.data.success) {
        setRows(normalized.map(createRow));
        setJsonText(JSON.stringify(normalized, null, 2));
        setJsonError('');
        showSuccess(t('保存成功'));
        await refresh?.();
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (e) {
      setJsonError(e.message);
      showError(e.message);
    } finally {
      setSaving(false);
    }
  };

  const columns = [
    {
      title: t('模型名称'),
      dataIndex: 'model',
      width: 220,
      render: (value, record) => (
        <Input
          value={value}
          placeholder='kling-v1'
          onChange={(val) => updateRow(record.id, 'model', val)}
        />
      ),
    },
    {
      title: 'mode',
      dataIndex: 'mode',
      width: 150,
      render: (value, record) => (
        <Input
          value={value}
          placeholder='std'
          onChange={(val) => updateRow(record.id, 'mode', val)}
        />
      ),
    },
    {
      title: 'sound',
      dataIndex: 'sound',
      width: 150,
      render: (value, record) => (
        <Input
          value={value}
          placeholder='off'
          onChange={(val) => updateRow(record.id, 'sound', val)}
        />
      ),
    },
    {
      title: t('每秒价格') + ' (USD)',
      dataIndex: 'price_per_second',
      width: 170,
      render: (value, record) => (
        <InputNumber
          value={value === '' ? undefined : value}
          min={0}
          step={0.001}
          onChange={(val) => updateRow(record.id, 'price_per_second', val ?? '')}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: '5s',
      width: 90,
      render: (_, record) => <Text>{formatPrice(record.price_per_second, 5)}</Text>,
    },
    {
      title: '10s',
      width: 90,
      render: (_, record) => <Text>{formatPrice(record.price_per_second, 10)}</Text>,
    },
    {
      title: t('操作'),
      width: 96,
      render: (_, record) => (
        <div style={{ display: 'flex', gap: 4 }}>
          <Button
            icon={<IconCopy />}
            theme='borderless'
            size='small'
            onClick={() => copyRow(record)}
          />
          <Button
            icon={<IconDelete />}
            type='danger'
            theme='borderless'
            size='small'
            onClick={() => removeRow(record.id)}
          />
        </div>
      ),
    },
  ];

  return (
    <div style={{ maxWidth: 1100 }}>
      <Banner
        type='warning'
        description={t('Kling 请求必须命中 model + mode + sound 的精确组合价，未配置组合会直接返回 400。')}
        style={{ marginBottom: 16 }}
      />

      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 8,
          alignItems: 'center',
          marginBottom: 12,
        }}
      >
        <RadioGroup
          type='button'
          size='small'
          value={mode}
          onChange={(e) => setMode(e.target.value)}
        >
          <Radio value='visual'>{t('可视化')}</Radio>
          <Radio value='json'>JSON</Radio>
        </RadioGroup>

        <Input
          value={search}
          prefix={<IconSearch size={14} />}
          placeholder={t('搜索模型/mode/sound')}
          onChange={setSearch}
          style={{ width: 220 }}
        />

        <Button icon={<IconPlus />} onClick={() => addRow()}>
          {t('添加')}
        </Button>
        {COMMON_MODELS.map((modelName) => (
          <Button
            key={modelName}
            size='small'
            theme='borderless'
            onClick={() => addRow(modelName)}
          >
            {modelName}
          </Button>
        ))}
      </div>

      {mode === 'visual' ? (
        <>
          <Table
            dataSource={filteredRows}
            columns={columns}
            pagination={false}
            rowKey='id'
            size='small'
            scroll={{ x: 966 }}
          />
          {validationError && rows.length > 0 && (
            <Text type='danger' size='small' style={{ display: 'block', marginTop: 8 }}>
              {validationError}
            </Text>
          )}
        </>
      ) : (
        <>
          <TextArea
            value={jsonText}
            onChange={syncToVisual}
            autosize={{ minRows: 10, maxRows: 24 }}
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
          {jsonError && (
            <Text type='danger' size='small' style={{ display: 'block', marginTop: 4 }}>
              {jsonError}
            </Text>
          )}
          <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
            <Button
              icon={<IconCopy />}
              size='small'
              theme='borderless'
              onClick={() => {
                copy(jsonText, t('JSON'));
              }}
            >
              {t('复制')}
            </Button>
          </div>
        </>
      )}

      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16 }}>
        <Button
          theme='solid'
          type='primary'
          loading={saving}
          disabled={mode === 'json' ? !!jsonError : !!validationError}
          onClick={handleSave}
        >
          {t('保存')}
        </Button>
      </div>
    </div>
  );
}
