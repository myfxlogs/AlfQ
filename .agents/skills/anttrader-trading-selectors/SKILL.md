---
name: anttrader-trading-selectors
description: >
  Reusable pattern for adding trading account and symbol pickers to AntTrader
  React pages. Use when building/modifying a page that needs: (1) a dropdown to
  select a trading account fetched from the backend, (2) a symbol dropdown
  that fetches live symbols from the broker via marketApi.getSymbols when an
  account is selected, (3) inline form layout with labels above controls, or
  (4) error handling with Alert + retry button when symbol fetch fails.
---

# AntTrader Trading Selectors

Common pattern for account picker + symbol picker forms in the AntTrader
frontend. Use this skill when adding account/symbol selection to any page.

## Quick Start

```tsx
import { marketApi } from '@/client/market';
import { researchClient } from '@/modules/research/client/research';

// 1. State
const [accounts, setAccounts] = useState<any[]>([]);
const [accLoading, setAccLoading] = useState(false);
const [symbols, setSymbols] = useState<{ value: string; label: string }[]>([]);
const [symLoading, setSymLoading] = useState(false);
const [symError, setSymError] = useState<string | null>(null);
const selectedAccountId = Form.useWatch('accountId', form);

// 2. Fetch accounts
const fetchAccounts = useCallback(async () => {
  setAccLoading(true);
  try {
    const r = await researchClient.dataset.listAccounts({});
    const list = (r.accounts || []).filter((a: any) => !a.isDisabled).map((a: any) => ({
      id: a.accountId, login: a.accountNumber, brokerCompany: a.broker || '',
      alias: a.accountNumber,
    }));
    setAccounts(list);
  } catch { setAccounts([]); }
  setAccLoading(false);
}, []);
useEffect(() => { fetchAccounts(); }, [fetchAccounts]);

// 3. Fetch symbols (live from broker, same as experiments page)
const fetchSymbols = useCallback(async () => {
  if (!selectedAccountId) { setSymbols([]); setSymError(null); return; }
  setSymLoading(true);
  setSymError(null);
  try {
    const list = await marketApi.getSymbols(selectedAccountId);
    const seen = new Set<string>();
    const opts = (list || [])
      .map((s: any) => String(s?.symbol || '').trim())
      .filter((v: string) => v)
      .filter((v: string) => { if (seen.has(v)) return false; seen.add(v); return true; })
      .map((v: string) => ({ value: v, label: v }));
    setSymbols(opts);
  } catch (e: any) {
    setSymbols([]);
    setSymError(e?.message || String(e) || 'Failed to fetch symbols');
  }
  setSymLoading(false);
}, [selectedAccountId]);
useEffect(() => { fetchSymbols(); }, [fetchSymbols]);

// 4. Clear symbol when account changes
useEffect(() => { form.setFieldsValue({ symbol: undefined }); }, [selectedAccountId, form]);
```

```tsx
{/* 5. Form layout: vertical (labels above), Space wrap (inline row) */}
<Form form={form} layout="vertical" onFinish={handleSubmit}>
  <Space size="large" wrap align="start">

    {/* Account picker */}
    <Form.Item name="accountId" label="Account"
      rules={[{ required: true }]}
      help={accounts.length === 0 ? 'No accounts available' : undefined}>
      <Select style={{ minWidth: 200, maxWidth: 260 }}
        options={accountOptions} loading={accLoading} showSearch allowClear />
    </Form.Item>

    {/* Symbol picker with retry on error */}
    <Form.Item name="symbol" label="Symbol"
      rules={[{ required: true }]}
      help={!selectedAccountId ? 'Select an account first' : undefined}>
      {symError ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 200 }}>
          <Alert type="error" message={symError} style={{ flex: 1, padding: '4px 12px' }} />
          <Button size="small" onClick={fetchSymbols} loading={symLoading}>Retry</Button>
        </div>
      ) : (
        <Select style={{ minWidth: 140, maxWidth: 200 }}
          options={symbols}
          disabled={!selectedAccountId} loading={symLoading}
          showSearch allowClear />
      )}
    </Form.Item>

    {/* Submit button: use label=" " for vertical alignment */}
    <Form.Item label=" ">
      <Button type="primary" htmlType="submit" disabled={!selectedAccountId}>
        Submit
      </Button>
    </Form.Item>

  </Space>
</Form>
```

## Key Design Decisions

- **Account source**: `researchClient.dataset.listAccounts` — returns accounts the user owns,
  filtered by `!isDisabled`. Each entry has `accountId`, `accountNumber`, `broker`, `isDisabled`, `isConnected`.
- **Symbol source**: `marketApi.getSymbols(accountId)` — live broker fetch (requires MT4/MT5 connection).
  Never use DB queries (`researchClient.dataset.listSymbols`) for user-facing pickers;
  that endpoint reads `kline_data` and is a static cache, not live broker data.
- **symbols type**: `{ value: string; label: string }[]` — matches the Select `options` prop directly,
  no inline `.map()` needed in JSX.
- **Error handling**: Replace the Select with `<Alert>` + retry `<Button>` when fetch fails.
  `symError` is `string | null`; the Alert shows the actual error message, not a hardcoded fallback.
- **Button alignment**: Give the submit button's `Form.Item` a `label=" "` (non-breaking space)
  so it aligns vertically with the other controls in `layout="vertical"`+`<Space>`.

## Files

- [UI pattern reference](references/ui-pattern.md) — full DatasetsPage example
- [API reference](references/api-reference.md) — relevant Connect RPC endpoints and TypeScript types
