---
name: anttrader-research-page-pattern
description: >
  Patterns and conventions for building AntTrader research module frontend pages.
  Use when building/modifying any research page under pages/research/ or pages/strategy/
  that needs: (1) CRUD table with inline forms, (2) ConnectRPC client calls via researchClient,
  (3) i18n labels from resources/zh-cn/research.ts, (4) deferEffect for setState-in-effect,
  (5) antd Table + Tag + Popconfirm patterns, or (6) proto-backed data structures.
---

# AntTrader Research Page Pattern

Conventions and reusable patterns for all research module frontend pages.
Follow these patterns for consistency across all `/research/*` tabs.

## Directory Convention

- Pages: `frontend/src/pages/research/<Name>Page.tsx` (≤ 600 lines)
- Client: `frontend/src/modules/research/client/research.ts` (aggregated ConnectRPC clients)
- i18n: `frontend/src/i18n/resources/zh-cn/research.ts`
- Routing: `frontend/src/pages/strategy/ResearchShell.tsx` (TABS array + conditional rendering)

## Standard Page Skeleton

Every research page follows this structure:

```tsx
import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Typography, Button, Space, Card, Form, message, Popconfirm } from 'antd';
import { ReloadOutlined, PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { researchClient } from '@/modules/research/client/research';
import { deferEffect } from '@/pages/strategy/lib/deferEffect';

const { Text } = Typography;

const MyPage: React.FC = () => {
  const { t } = useTranslation();

  // ── Data state ──
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  // ── Fetch ──
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const r = await researchClient.xxx.listXxx({ limit: 50 });
      setData(r.xxx || []);  // proto returns plural field
    } catch { setData([]); }
    setLoading(false);
  }, []);
  useEffect(() => { deferEffect(fetchData); }, [fetchData]);

  // ── Delete ──
  const handleDelete = async (id: string) => {
    try {
      await researchClient.xxx.deleteXxx({ xxxId: id });
      message.success(t('research.xxx.deleteSuccess', '删除成功'));
      fetchData();
    } catch (e: any) {
      message.error(e?.message || t('research.xxx.deleteFailed', '删除失败'));
    }
  };

  return (
    <div>
      <Card size="small" style={{ marginBottom: 12 }}>
        {/* Create form goes here */}
      </Card>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <Text strong>{t('research.tabs.xxx')}</Text>
        <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
          {t('research.xxx.refresh', '刷新')}
        </Button>
      </div>
      <Table size="small" dataSource={data} rowKey="id" loading={loading}
        columns={[/* ... */]} />
    </div>
  );
};
export default MyPage;
```

## ConnectRPC Client Patterns

All research pages use `researchClient` from `@/modules/research/client/research.ts`.
The client exposes 12 services:

```ts
researchClient.dataset    // ResearchDatasetService
researchClient.factor     // ResearchFactorService
researchClient.signal     // ResearchSignalService
researchClient.model      // ResearchModelService
researchClient.robustness // ResearchRobustnessService
researchClient.report     // ResearchReportService
researchClient.hypothesis // ResearchHypothesisService
researchClient.task       // ResearchTaskService
researchClient.universe   // ResearchUniverseService
researchClient.lineage    // ResearchLineageService
researchClient.featureSet // ResearchFeatureSetService
researchClient.paperTrading // ResearchPaperTradingService
```

Call convention: `researchClient.<service>.<rpcName>({...fields})`

### Proto field naming

Proto `snake_case` fields become `camelCase` in the JS client:
- `researchClient.universe.listUniverses({ limit: 50 })` → response `.universes`
- `researchClient.hypothesis.listHypotheses({ limit: 50 })` → response `.hypotheses`
- `researchClient.featureSet.listFeatureSets({ limit: 50 })` → response `.featureSets`

## Create Form Pattern (Card + Form + Space)

```tsx
import { Form, Input, Select, Button, Card, Space, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';

// Inside component:
const [form] = Form.useForm();
const [creating, setCreating] = useState(false);

const handleCreate = async (values: any) => {
  setCreating(true);
  try {
    await researchClient.xxx.createXxx(values);
    message.success(t('research.xxx.createSuccess'));
    form.resetFields();
    fetchData();
  } catch (e: any) {
    message.error(e?.message || t('research.xxx.createFailed'));
  }
  setCreating(false);
};

// JSX:
<Card size="small" style={{ marginBottom: 12 }}>
  <Form form={form} layout="inline" onFinish={handleCreate}>
    <Form.Item name="name" rules={[{ required: true }]}>
      <Input placeholder={t('research.xxx.name')} />
    </Form.Item>
    <Form.Item>
      <Button type="primary" htmlType="submit" icon={<PlusOutlined />} loading={creating}>
        {t('research.xxx.create')}
      </Button>
    </Form.Item>
  </Form>
</Card>
```

## Table Columns Pattern

```tsx
const columns = [
  {
    title: 'ID', dataIndex: 'id', width: 100,
    render: (v: string) => <Text code copyable>{v?.slice(0, 8)}</Text>
  },
  {
    title: t('research.xxx.name'), dataIndex: 'name',
  },
  {
    title: t('research.xxx.status'), dataIndex: 'status', width: 100,
    render: (v: string) => <Tag color={v === 'approved' ? 'green' : v === 'draft' ? 'default' : 'gold'}>{v}</Tag>
  },
  {
    title: '', width: 50,
    render: (_: any, record: any) => (
      <Popconfirm title={t('research.xxx.confirmDelete')}
        onConfirm={() => handleDelete(record.id)}>
        <Button size="small" danger icon={<DeleteOutlined />} />
      </Popconfirm>
    ),
  },
];
```

## i18n Convention

All labels use `t('research.<section>.<key>', 'fallback')`:
- Section matches the resource file section: `dataset`, `universe`, `hypothesis`, etc.
- Each page typically adds `.create`, `.createSuccess`, `.createFailed`, `.confirmDelete`
- Import: `import { useTranslation } from 'react-i18next';`

## Key React Patterns

1. **deferEffect**: Wrap every `setState` call in `useEffect` with `deferEffect`:
   ```ts
   import { deferEffect } from '@/pages/strategy/lib/deferEffect';
   useEffect(() => { deferEffect(fetchData); }, [fetchData]);
   ```

2. **useCallback for fetch functions**: All data-fetching functions wrapped in `useCallback`.

3. **Try-catch with setState fallback**: Every fetch sets state to `[]` on error.

4. **Row key**: Always `rowKey="id"` or `rowKey="<protoIdField>"` matching proto field name.

For a complete example, see [references/datasets-page.md](references/datasets-page.md).
