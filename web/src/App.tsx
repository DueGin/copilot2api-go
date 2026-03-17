import { useCallback, useEffect, useState } from "react"

import { api, getSessionToken, setSessionToken, type Account, type BatchUsageItem, type CopilotModel, type ModelMapping, type Pool, type ProxySettings, type ProxyUsageSnapshot } from "./api"
import { AccountCard } from "./components/AccountCard"
import { AddAccountForm } from "./components/AddAccountForm"
import { CopyableSecret } from "./components/CopyableSecret"
import { useLocale, useT } from "./i18n"

type AuthState = "loading" | "setup" | "login" | "authed"

function LanguageSwitcher() {
  const { locale, setLocale } = useLocale()
  return (
    <button
      onClick={() => setLocale(locale === "en" ? "zh" : "en")}
      style={{ fontSize: 13, padding: "4px 10px" }}
    >
      {locale === "en" ? "中文" : "EN"}
    </button>
  )
}

function SetupForm({ onComplete }: { onComplete: () => void }) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const t = useT()

  const handleSubmit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError("")
    if (password !== confirm) { setError(t("passwordMismatch")); return }
    if (password.length < 6) { setError(t("passwordTooShort")); return }
    setLoading(true)
    try {
      const { token } = await api.setup(username, password)
      setSessionToken(token)
      onComplete()
    } catch (err) { setError((err as Error).message) }
    finally { setLoading(false) }
  }

  return (
    <div style={{ maxWidth: 400, margin: "120px auto", padding: "0 16px" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600 }}>{t("consoleTitle")}</h1>
        <LanguageSwitcher />
      </div>
      <p style={{ color: "var(--text-muted)", fontSize: 14, marginBottom: 24 }}>{t("setupSubtitle")}</p>
      <form onSubmit={(e) => void handleSubmit(e)}>
        <input type="text" value={username} onChange={(e) => setUsername(e.target.value)} placeholder={t("usernamePlaceholder")} autoFocus autoComplete="username" style={{ marginBottom: 12 }} />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("passwordPlaceholder")} autoComplete="new-password" style={{ marginBottom: 12 }} />
        <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} placeholder={t("confirmPasswordPlaceholder")} autoComplete="new-password" style={{ marginBottom: 12 }} />
        {error && <div style={{ color: "var(--red)", fontSize: 13, marginBottom: 12 }}>{error}</div>}
        <button type="submit" className="primary" disabled={loading}>{loading ? t("creating") : t("createAdmin")}</button>
      </form>
    </div>
  )
}

function LoginForm({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const t = useT()

  const handleSubmit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError("")
    setLoading(true)
    try {
      const { token } = await api.login(username, password)
      setSessionToken(token)
      onLogin()
    } catch { setError(t("invalidCredentials")) }
    finally { setLoading(false) }
  }

  return (
    <div style={{ maxWidth: 400, margin: "120px auto", padding: "0 16px" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600 }}>{t("consoleTitle")}</h1>
        <LanguageSwitcher />
      </div>
      <p style={{ color: "var(--text-muted)", fontSize: 14, marginBottom: 24 }}>{t("loginSubtitle")}</p>
      <form onSubmit={(e) => void handleSubmit(e)}>
        <input type="text" value={username} onChange={(e) => setUsername(e.target.value)} placeholder={t("usernamePlaceholder")} autoFocus autoComplete="username" style={{ marginBottom: 12 }} />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("passwordPlaceholder")} autoComplete="current-password" style={{ marginBottom: 12 }} />
        {error && <div style={{ color: "var(--red)", fontSize: 13, marginBottom: 12 }}>{error}</div>}
        <button type="submit" className="primary" disabled={loading}>{loading ? t("signingIn") : t("signIn")}</button>
      </form>
    </div>
  )
}

function AccountList({ accounts, proxyPort, pools, onRefresh }: { accounts: Array<Account>; proxyPort: number; pools: Array<Pool>; onRefresh: () => Promise<void> }) {
  const t = useT()

  // Build account -> pool name lookup
  const accountPoolMap: Record<string, string> = {}
  for (const pool of pools) {
    for (const aid of pool.accountIds ?? []) {
      accountPoolMap[aid] = pool.name
    }
  }

  if (accounts.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: 60, color: "var(--text-muted)", border: "1px dashed var(--border)", borderRadius: "var(--radius)" }}>
        <p style={{ fontSize: 16, marginBottom: 8 }}>{t("noAccounts")}</p>
        <p style={{ fontSize: 13 }}>{t("noAccountsHint")}</p>
      </div>
    )
  }
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {accounts.map((account) => (
        <AccountCard key={account.id} account={account} proxyPort={proxyPort} poolName={accountPoolMap[account.id]} onRefresh={onRefresh} />
      ))}
    </div>
  )
}

const strategyKeys = ["round-robin", "priority", "least-used", "smart"] as const
const strategyLabelMap: Record<string, "roundRobin" | "priority" | "leastUsed" | "smart"> = {
  "round-robin": "roundRobin",
  priority: "priority",
  "least-used": "leastUsed",
  smart: "smart",
}
const strategyDescMap: Record<string, "roundRobinDesc" | "priorityDesc" | "leastUsedDesc" | "smartDesc"> = {
  "round-robin": "roundRobinDesc",
  priority: "priorityDesc",
  "least-used": "leastUsedDesc",
  smart: "smartDesc",
}

function ProxySettingsPanel({ settings, onChange }: { settings: ProxySettings; onChange: (s: ProxySettings) => void }) {
  const [input, setInput] = useState(settings.proxyURL ?? "")
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const t = useT()

  useEffect(() => {
    setInput(settings.proxyURL ?? "")
  }, [settings.proxyURL])

  const save = async (url: string) => {
    setSaving(true)
    try {
      const updated = await api.updateProxySettings({ proxyURL: url })
      onChange(updated)
      setSaved(true)
      setTimeout(() => setSaved(false), 1500)
    } finally {
      setSaving(false)
    }
  }

  const handleBlur = () => {
    if (!saving && input !== settings.proxyURL) void save(input)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") void save(input)
  }

  const handleClear = () => {
    setInput("")
    void save("")
  }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 16 }}>
      <div style={{ fontSize: 15, fontWeight: 600, marginBottom: 4 }}>{t("proxySettings")}</div>
      <div style={{ fontSize: 13, color: "var(--text-muted)", marginBottom: 10 }}>{t("proxySettingsDesc")}</div>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span style={{ fontSize: 13, color: "var(--text-muted)", flexShrink: 0 }}>{t("proxyUrlLabel")}</span>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onBlur={handleBlur}
          onKeyDown={handleKeyDown}
          placeholder={t("proxyUrlPlaceholder")}
          style={{ flex: 1, fontSize: 13, padding: "4px 8px", fontFamily: "monospace" }}
        />
        {input && (
          <button type="button" onClick={handleClear} disabled={saving} style={{ padding: "4px 10px", fontSize: 12 }}>
            {t("proxyClear")}
          </button>
        )}
        <button type="button" className="primary" onClick={() => void save(input)} disabled={saving} style={{ padding: "4px 10px", fontSize: 12, flexShrink: 0 }}>
          {saved ? t("proxySaved") : saving ? t("saving") : t("save")}
        </button>
      </div>
    </div>
  )
}

function PoolCard({ pool, accounts, proxyPort, onUpdate }: { pool: Pool; accounts: Array<Account>; proxyPort: number; onUpdate: () => void }) {
  const [saving, setSaving] = useState(false)
  const [keyVisible, setKeyVisible] = useState(false)
  const [rpmInput, setRpmInput] = useState(String(pool.rateLimitRPM ?? 0))
  const [editingName, setEditingName] = useState(false)
  const [nameValue, setNameValue] = useState(pool.name)
  const [editingStrategy, setEditingStrategy] = useState(false)
  const [editingProxy, setEditingProxy] = useState(false)
  const [proxyInput, setProxyInput] = useState(pool.proxyURL ?? "")
  const [confirmDelete, setConfirmDelete] = useState(false)
  const t = useT()

  const maskedKey = pool.apiKey?.length > 8 ? `${pool.apiKey.slice(0, 8)}${"•".repeat(24)}` : pool.apiKey ?? ""
  const proxyBase = `${window.location.protocol}//${window.location.hostname}:${proxyPort}`

  const poolAccountIds = new Set(pool.accountIds ?? [])
  const poolAccounts = accounts.filter(a => poolAccountIds.has(a.id))
  const availableAccounts = accounts.filter(a => !poolAccountIds.has(a.id))

  const toggle = async () => { setSaving(true); try { await api.updatePool(pool.id, { enabled: !pool.enabled }); onUpdate() } finally { setSaving(false) } }
  const changeStrategy = async (strategy: string) => { setSaving(true); try { await api.updatePool(pool.id, { strategy }); onUpdate(); setEditingStrategy(false) } finally { setSaving(false) } }
  const regenKey = async () => { setSaving(true); try { await api.regeneratePoolKey(pool.id); onUpdate() } finally { setSaving(false) } }

  const saveRPM = async () => {
    const num = parseInt(rpmInput, 10)
    if (isNaN(num) || num < 0) { setRpmInput(String(pool.rateLimitRPM ?? 0)); return }
    if (num !== (pool.rateLimitRPM ?? 0)) {
      setSaving(true)
      try { await api.updatePool(pool.id, { rateLimitRPM: num }); onUpdate() }
      finally { setSaving(false) }
    }
  }

  const handleNameSave = async () => {
    const trimmed = nameValue.trim()
    if (!trimmed) { setNameValue(pool.name); setEditingName(false); return }
    setEditingName(false)
    if (trimmed !== pool.name) {
      setSaving(true)
      try { await api.updatePool(pool.id, { name: trimmed }); onUpdate() }
      finally { setSaving(false) }
    }
  }

  const handleProxySave = async () => {
    setEditingProxy(false)
    if (proxyInput !== (pool.proxyURL ?? "")) {
      setSaving(true)
      try { await api.updatePool(pool.id, { proxyURL: proxyInput }); onUpdate() }
      finally { setSaving(false) }
    }
  }

  const addAccount = async (accountId: string) => {
    setSaving(true)
    try { await api.addAccountToPool(pool.id, accountId); onUpdate() }
    catch (err) { console.error("Add account to pool failed:", err) }
    finally { setSaving(false) }
  }

  const removeAccount = async (accountId: string) => {
    setSaving(true)
    try { await api.removeAccountFromPool(pool.id, accountId); onUpdate() }
    finally { setSaving(false) }
  }

  const handleDelete = async () => {
    if (!confirmDelete) { setConfirmDelete(true); setTimeout(() => setConfirmDelete(false), 3000); return }
    setSaving(true)
    try { await api.deletePool(pool.id); onUpdate() }
    finally { setSaving(false); setConfirmDelete(false) }
  }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 12 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {editingName ? (
            <input
              type="text" value={nameValue} onChange={(e) => setNameValue(e.target.value)}
              onBlur={() => void handleNameSave()}
              onKeyDown={(e) => { if (e.key === "Enter") void handleNameSave(); if (e.key === "Escape") { setNameValue(pool.name); setEditingName(false) } }}
              autoFocus style={{ fontSize: 15, fontWeight: 600, padding: "2px 6px", width: 200 }}
            />
          ) : (
            <span style={{ fontSize: 15, fontWeight: 600, cursor: "pointer" }} onClick={() => { setNameValue(pool.name); setEditingName(true) }} title={t("editName")}>
              {pool.name}
            </span>
          )}
          <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: pool.enabled ? "var(--green)" : "var(--border)", color: pool.enabled ? "#fff" : "var(--text-muted)" }}>
            {pool.enabled ? t("enable") : t("disable")}
          </span>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button onClick={() => void toggle()} disabled={saving} style={{ fontSize: 12 }}>{pool.enabled ? t("disable") : t("enable")}</button>
          <button className="danger" onClick={() => void handleDelete()} disabled={saving} style={{ fontSize: 12 }}>{confirmDelete ? t("confirmDeletePool") : t("delete")}</button>
        </div>
      </div>

      {pool.enabled && (
        <>
          {/* Strategy */}
          <div style={{ marginTop: 10, display: "flex", alignItems: "center", gap: 8, fontSize: 13, flexWrap: "wrap" }}>
            {editingStrategy ? (
              <>
                {strategyKeys.map((s) => (
                  <button key={s} className={pool.strategy === s ? "primary" : undefined} onClick={() => void changeStrategy(s)} disabled={saving || pool.strategy === s} style={{ fontSize: 12 }}>
                    {t(strategyLabelMap[s])}
                  </button>
                ))}
                <button onClick={() => setEditingStrategy(false)} style={{ fontSize: 11 }}>{t("cancel")}</button>
              </>
            ) : (
              <span style={{ color: "var(--text-muted)", cursor: "pointer" }} onClick={() => setEditingStrategy(true)}>
                {t(strategyLabelMap[pool.strategy] ?? "roundRobin")} — {t(strategyDescMap[pool.strategy] ?? "roundRobinDesc")}
              </span>
            )}
          </div>

          {/* Rate Limit */}
          <div style={{ marginTop: 8, display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
            <span style={{ color: "var(--text-muted)", flexShrink: 0 }}>{t("rateLimitRPM")}</span>
            <input type="number" value={rpmInput} onChange={(e) => setRpmInput(e.target.value)}
              onBlur={() => void saveRPM()} onKeyDown={(e) => { if (e.key === "Enter") void saveRPM() }}
              min={0} style={{ width: 80, padding: "4px 8px", fontSize: 13 }} placeholder={t("rateLimitPlaceholder")} />
            <span style={{ fontSize: 12, color: "var(--text-muted)" }}>{t("rateLimitRPMDesc")}</span>
          </div>

          {/* Proxy URL */}
          <div style={{ marginTop: 8, display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
            <span style={{ color: "var(--text-muted)", flexShrink: 0 }}>{t("poolProxy")}</span>
            {editingProxy ? (
              <>
                <input type="text" value={proxyInput} onChange={(e) => setProxyInput(e.target.value)}
                  onBlur={() => void handleProxySave()} onKeyDown={(e) => { if (e.key === "Enter") void handleProxySave() }}
                  placeholder={t("poolProxyUrlPlaceholder")} style={{ flex: 1, fontSize: 13, padding: "4px 8px", fontFamily: "monospace" }} autoFocus />
              </>
            ) : (
              <span style={{ fontFamily: "monospace", cursor: "pointer", padding: "2px 8px", background: "var(--bg)", border: "1px solid var(--border)", borderRadius: 4 }}
                onClick={() => { setProxyInput(pool.proxyURL ?? ""); setEditingProxy(true) }}>
                {pool.proxyURL || t("noProxy")}
              </span>
            )}
          </div>

          {/* API Key */}
          <div style={{ marginTop: 10, padding: 10, background: "var(--bg)", borderRadius: "var(--radius)", fontSize: 12, fontFamily: "monospace", display: "flex", alignItems: "center", gap: 8 }}>
            <CopyableSecret idleLabel={t("poolKey")} copiedLabel={t("copied")} secret={pool.apiKey ?? ""} maskedSecret={maskedKey} visible={keyVisible} copyTitle={t("clickToCopy")} />
            <button type="button" onClick={() => setKeyVisible(!keyVisible)} style={{ padding: "2px 8px", fontSize: 11 }}>{keyVisible ? t("hide") : t("show")}</button>
            <button type="button" onClick={() => void regenKey()} disabled={saving} style={{ padding: "2px 8px", fontSize: 11 }}>{t("regen")}</button>
          </div>
          <div style={{ marginTop: 4, fontSize: 12, color: "var(--text-muted)", fontFamily: "monospace" }}>
            {t("baseUrl")} {proxyBase} &nbsp;·&nbsp; Bearer {pool.apiKey?.slice(0, 8)}...
          </div>

          {/* Pool Accounts */}
          <div style={{ marginTop: 12 }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>{t("poolAccounts")} ({poolAccounts.length})</div>
            {poolAccounts.length === 0 ? (
              <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 8 }}>{t("noAccountsInPool")}</div>
            ) : (
              <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                {poolAccounts.map(a => (
                  <span key={a.id} style={{ display: "inline-flex", alignItems: "center", gap: 4, padding: "3px 8px", background: "var(--bg)", border: "1px solid var(--border)", borderRadius: 4, fontSize: 12 }}>
                    {a.name}
                    <span onClick={() => void removeAccount(a.id)} style={{ cursor: "pointer", color: "var(--red)", fontWeight: 600, marginLeft: 2 }} title={t("removeFromPool")}>×</span>
                  </span>
                ))}
              </div>
            )}
            {availableAccounts.length > 0 && (
              <div style={{ marginTop: 8, display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ fontSize: 12, color: "var(--text-muted)" }}>{t("addToPool")}:</span>
                <select onChange={(e) => { if (e.target.value) { void addAccount(e.target.value); e.target.value = "" } }} disabled={saving} style={{ fontSize: 12, padding: "3px 6px" }}>
                  <option value="">-- {t("availableAccounts")} --</option>
                  {availableAccounts.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
                </select>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}

function CreatePoolForm({ onCreated, onCancel }: { onCreated: () => void; onCancel: () => void }) {
  const [name, setName] = useState("")
  const [strategy, setStrategy] = useState("round-robin")
  const [proxyURL, setProxyURL] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const t = useT()

  const handleSubmit = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError(t("accountNameRequired")); return }
    setLoading(true)
    setError("")
    try {
      await api.createPool({ name: name.trim(), strategy, proxyURL: proxyURL.trim() || undefined })
      onCreated()
    } catch (err) { setError((err as Error).message) }
    finally { setLoading(false) }
  }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 12 }}>
      <form onSubmit={(e) => void handleSubmit(e)}>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "flex-end" }}>
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>{t("poolName")}</div>
            <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("poolNamePlaceholder")} style={{ fontSize: 13, padding: "4px 8px", width: 200 }} autoFocus />
          </div>
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>{t("poolProxyUrl")}</div>
            <input type="text" value={proxyURL} onChange={(e) => setProxyURL(e.target.value)} placeholder={t("poolProxyUrlPlaceholder")} style={{ fontSize: 13, padding: "4px 8px", width: 260, fontFamily: "monospace" }} />
          </div>
          <div>
            <select value={strategy} onChange={(e) => setStrategy(e.target.value)} style={{ fontSize: 13, padding: "5px 8px" }}>
              {strategyKeys.map(s => <option key={s} value={s}>{t(strategyLabelMap[s])}</option>)}
            </select>
          </div>
          <button type="submit" className="primary" disabled={loading} style={{ fontSize: 13 }}>{loading ? t("creating") : t("createPool")}</button>
          <button type="button" onClick={onCancel} style={{ fontSize: 13 }}>{t("cancel")}</button>
        </div>
        {error && <div style={{ color: "var(--red)", fontSize: 12, marginTop: 8 }}>{error}</div>}
      </form>
    </div>
  )
}

function PoolManagement({ accounts, proxyPort }: { accounts: Array<Account>; proxyPort: number }) {
  const [pools, setPools] = useState<Array<Pool>>([])
  const [showCreate, setShowCreate] = useState(false)
  const [fetched, setFetched] = useState(false)
  const t = useT()

  const fetchPools = async () => {
    try { const data = await api.getPools(); setPools(data); setFetched(true) }
    catch (err) { console.error("Failed to fetch pools:", err) }
  }

  useEffect(() => { void fetchPools() }, [])

  const handleUpdate = () => { void fetchPools() }
  const handleCreated = () => { setShowCreate(false); void fetchPools() }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
        <div>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{t("poolManagement")}</div>
          <div style={{ fontSize: 13, color: "var(--text-muted)" }}>{t("poolEnabledDesc")}</div>
        </div>
        <button className="primary" onClick={() => setShowCreate(!showCreate)} style={{ flexShrink: 0 }}>
          {showCreate ? t("cancel") : t("createPool")}
        </button>
      </div>
      {showCreate && <CreatePoolForm onCreated={handleCreated} onCancel={() => setShowCreate(false)} />}
      {fetched && pools.length === 0 && !showCreate && (
        <div style={{ textAlign: "center", padding: 24, color: "var(--text-muted)", fontSize: 13 }}>
          <p>{t("noPools")}</p>
          <p style={{ fontSize: 12 }}>{t("noPoolsHint")}</p>
        </div>
      )}
      {pools.map(pool => (
        <PoolCard key={pool.id} pool={pool} accounts={accounts} proxyPort={proxyPort} onUpdate={handleUpdate} />
      ))}
    </div>
  )
}

function usageColor(pct: number): string {
  if (pct > 90) return "var(--red)"
  if (pct > 70) return "var(--yellow)"
  return "var(--green)"
}

function UsageCell({ used, total, unlimited }: { used: number; total: number; unlimited?: boolean }) {
  if (unlimited) {
    return (
      <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace" }}>
        <span style={{ color: "var(--green)" }}>∞</span>
      </td>
    )
  }
  const pct = total > 0 ? (used / total) * 100 : 0
  return (
    <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace" }}>
      <span style={{ color: usageColor(pct) }}>{used}</span>
      <span style={{ color: "var(--text-muted)" }}> / {total}</span>
    </td>
  )
}

function BatchUsagePanel() {
  const [items, setItems] = useState<Array<BatchUsageItem>>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [fetched, setFetched] = useState(false)
  const t = useT()

  const fetchAll = async () => {
    setLoading(true)
    try { const data = await api.getAllUsage(); setItems(data); setFetched(true); setOpen(true) }
    catch (err) { console.error("Batch usage failed:", err) }
    finally { setLoading(false) }
  }

  const runningItems = items.filter((i) => i.usage?.quota_snapshots)
  const totals = runningItems.reduce(
    (acc, i) => {
      const q = i.usage!.quota_snapshots
      if (!q.premium_interactions?.unlimited) {
        acc.premiumUsed += (q.premium_interactions?.entitlement ?? 0) - (q.premium_interactions?.remaining ?? 0)
        acc.premiumTotal += q.premium_interactions?.entitlement ?? 0
      } else {
        acc.premiumUnlimited = true
      }
      if (!q.chat?.unlimited) {
        acc.chatUsed += (q.chat?.entitlement ?? 0) - (q.chat?.remaining ?? 0)
        acc.chatTotal += q.chat?.entitlement ?? 0
      } else {
        acc.chatUnlimited = true
      }
      if (!q.completions?.unlimited) {
        acc.compUsed += (q.completions?.entitlement ?? 0) - (q.completions?.remaining ?? 0)
        acc.compTotal += q.completions?.entitlement ?? 0
      } else {
        acc.compUnlimited = true
      }
      return acc
    },
    { premiumUsed: 0, premiumTotal: 0, premiumUnlimited: false, chatUsed: 0, chatTotal: 0, chatUnlimited: false, compUsed: 0, compTotal: 0, compUnlimited: false },
  )

  const thStyle: React.CSSProperties = { padding: "8px 10px", textAlign: "left", fontSize: 12, fontWeight: 600, color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ fontSize: 15, fontWeight: 600 }}>{t("batchUsage")}</div>
        <div style={{ display: "flex", gap: 8 }}>
          <button className="primary" onClick={() => void fetchAll()} disabled={loading}>{loading ? t("refreshing") : t("queryAllUsage")}</button>
          {fetched && <button onClick={() => setOpen(!open)}>{open ? t("hide") : t("show")}</button>}
        </div>
      </div>
      {open && fetched && (
        <div style={{ marginTop: 12, overflowX: "auto" }}>
          {runningItems.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 16, textAlign: "center" }}>{t("noRunningAccounts")}</div>
          ) : (
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead><tr>
                <th style={thStyle}>{t("colAccount")}</th><th style={thStyle}>{t("colPlan")}</th>
                <th style={thStyle}>{t("colPremium")}</th><th style={thStyle}>{t("colChat")}</th>
                <th style={thStyle}>{t("colCompletions")}</th><th style={thStyle}>{t("colResets")}</th>
              </tr></thead>
              <tbody>
                {runningItems.map((item) => {
                  const q = item.usage!.quota_snapshots
                  return (
                    <tr key={item.accountId} style={{ borderBottom: "1px solid var(--border)" }}>
                      <td style={{ padding: "8px 10px", fontSize: 13, fontWeight: 500 }}>{item.name}</td>
                      <td style={{ padding: "8px 10px", fontSize: 12, color: "var(--text-muted)" }}>{item.usage!.copilot_plan}</td>
                      <UsageCell used={(q.premium_interactions?.entitlement ?? 0) - (q.premium_interactions?.remaining ?? 0)} total={q.premium_interactions?.entitlement ?? 0} unlimited={q.premium_interactions?.unlimited} />
                      <UsageCell used={(q.chat?.entitlement ?? 0) - (q.chat?.remaining ?? 0)} total={q.chat?.entitlement ?? 0} unlimited={q.chat?.unlimited} />
                      <UsageCell used={(q.completions?.entitlement ?? 0) - (q.completions?.remaining ?? 0)} total={q.completions?.entitlement ?? 0} unlimited={q.completions?.unlimited} />
                      <td style={{ padding: "8px 10px", fontSize: 12, color: "var(--text-muted)" }}>{item.usage!.quota_reset_date}</td>
                    </tr>
                  )
                })}
                <tr style={{ fontWeight: 600, borderTop: "2px solid var(--border)" }}>
                  <td style={{ padding: "8px 10px", fontSize: 13 }}>{t("totalSummary")}</td><td />
                  <UsageCell used={totals.premiumUsed} total={totals.premiumTotal} unlimited={totals.premiumUnlimited} />
                  <UsageCell used={totals.chatUsed} total={totals.chatTotal} unlimited={totals.chatUnlimited} />
                  <UsageCell used={totals.compUsed} total={totals.compTotal} unlimited={totals.compUnlimited} />
                  <td />
                </tr>
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}

function ProxyUsagePanel({ accounts }: { accounts: Array<Account> }) {
  const [data, setData] = useState<Record<string, ProxyUsageSnapshot>>({})
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [fetched, setFetched] = useState(false)
  const t = useT()

  const fetchUsage = async () => {
    setLoading(true)
    try { const d = await api.getProxyUsage(); setData(d); setFetched(true); setOpen(true) }
    catch (err) { console.error("Proxy usage fetch failed:", err) }
    finally { setLoading(false) }
  }

  const accountNameMap: Record<string, string> = {}
  for (const a of accounts) { accountNameMap[a.id] = a.name }

  const entries = Object.entries(data).filter(([, snap]) => snap.totalRequests > 0)
  const totalReqs = entries.reduce((sum, [, s]) => sum + s.totalRequests, 0)
  const totalFailed = entries.reduce((sum, [, s]) => sum + s.failedRequests, 0)

  const thStyle: React.CSSProperties = { padding: "8px 10px", textAlign: "left", fontSize: 12, fontWeight: 600, color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{t("proxyUsage")}</div>
          <div style={{ fontSize: 13, color: "var(--text-muted)" }}>{t("proxyUsageDesc")}</div>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button className="primary" onClick={() => void fetchUsage()} disabled={loading}>{loading ? t("refreshing") : t("queryProxyUsage")}</button>
          {fetched && <button onClick={() => setOpen(!open)}>{open ? t("hide") : t("show")}</button>}
        </div>
      </div>
      {open && fetched && (
        <div style={{ marginTop: 12, overflowX: "auto" }}>
          {entries.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 16, textAlign: "center" }}>{t("noProxyUsage")}</div>
          ) : (
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead><tr>
                <th style={thStyle}>{t("colAccount")}</th>
                <th style={thStyle}>{t("colTotalReqs")}</th>
                <th style={thStyle}>{t("colFailedReqs")}</th>
                <th style={thStyle}>{t("colLast429")}</th>
              </tr></thead>
              <tbody>
                {entries.map(([accountId, snap]) => (
                  <tr key={accountId} style={{ borderBottom: "1px solid var(--border)" }}>
                    <td style={{ padding: "8px 10px", fontSize: 13, fontWeight: 500 }}>{accountNameMap[accountId] ?? accountId.slice(0, 8)}</td>
                    <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace" }}>{snap.totalRequests}</td>
                    <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace", color: snap.failedRequests > 0 ? "var(--red)" : undefined }}>{snap.failedRequests}</td>
                    <td style={{ padding: "8px 10px", fontSize: 12, color: "var(--text-muted)" }}>
                      {snap.last429At ? new Date(snap.last429At).toLocaleTimeString() : t("never")}
                    </td>
                  </tr>
                ))}
                <tr style={{ fontWeight: 600, borderTop: "2px solid var(--border)" }}>
                  <td style={{ padding: "8px 10px", fontSize: 13 }}>{t("totalSummary")}</td>
                  <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace" }}>{totalReqs}</td>
                  <td style={{ padding: "8px 10px", fontSize: 12, fontFamily: "monospace", color: totalFailed > 0 ? "var(--red)" : undefined }}>{totalFailed}</td>
                  <td />
                </tr>
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}

function ModelMappingPanel() {
  const [mappings, setMappings] = useState<Array<ModelMapping>>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [open, setOpen] = useState(false)
  const [fetched, setFetched] = useState(false)
  const [copilotModels, setCopilotModels] = useState<Array<CopilotModel>>([])
  const [fetchingModels, setFetchingModels] = useState(false)
  const [modelsFetched, setModelsFetched] = useState(false)
  const t = useT()

  const fetchMappings = async () => {
    setLoading(true)
    try {
      const data = await api.getModelMappings()
      setMappings(data.mappings ?? [])
      setFetched(true)
      setOpen(true)
    } catch (err) { console.error("Failed to fetch model mappings:", err) }
    finally { setLoading(false) }
  }

  const saveMappings = async () => {
    setSaving(true)
    try {
      const valid = mappings.filter(m => m.copilotId && m.displayId)
      const data = await api.setModelMappings(valid)
      setMappings(data.mappings ?? [])
      // Refresh copilot models mapping status after saving
      if (modelsFetched) {
        void fetchCopilotModels()
      }
    } catch (err) { console.error("Failed to save model mappings:", err) }
    finally { setSaving(false) }
  }

  const fetchCopilotModels = async () => {
    setFetchingModels(true)
    try {
      const data = await api.getCopilotModels()
      setCopilotModels(data.models ?? [])
      setModelsFetched(true)
    } catch (err) { console.error("Failed to fetch copilot models:", err) }
    finally { setFetchingModels(false) }
  }

  const quickAddModel = (model: CopilotModel) => {
    // Don't add if already in the mappings list
    if (mappings.some(m => m.copilotId === model.id)) return
    setMappings([...mappings, { copilotId: model.id, displayId: "", displayName: "" }])
  }

  const addRow = () => setMappings([...mappings, { copilotId: "", displayId: "", displayName: "" }])
  const removeRow = (idx: number) => setMappings(mappings.filter((_, i) => i !== idx))
  const updateRow = (idx: number, field: keyof ModelMapping, value: string) => {
    const updated = [...mappings]
    updated[idx] = { ...updated[idx], [field]: value }
    setMappings(updated)
  }

  const thStyle: React.CSSProperties = { padding: "6px 10px", textAlign: "left", fontSize: 12, fontWeight: 600, color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }

  return (
    <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius)", padding: 16, marginBottom: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{t("modelMapping")}</div>
          <div style={{ fontSize: 13, color: "var(--text-muted)" }}>{t("modelMappingDesc")}</div>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          {!fetched && <button className="primary" onClick={() => void fetchMappings()} disabled={loading}>{loading ? t("loading") : t("show")}</button>}
          {fetched && <button onClick={() => setOpen(!open)}>{open ? t("hide") : t("show")}</button>}
        </div>
      </div>
      {open && fetched && (
        <div style={{ marginTop: 12 }}>
          {/* Copilot Models Section */}
          <div style={{ marginBottom: 16, padding: 12, background: "var(--bg)", borderRadius: "var(--radius)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 13, fontWeight: 600 }}>{t("copilotModels")}</span>
              <button onClick={() => void fetchCopilotModels()} disabled={fetchingModels} style={{ fontSize: 12, padding: "4px 12px" }}>
                {fetchingModels ? t("fetchingModels") : t("fetchModels")}
              </button>
            </div>
            {modelsFetched && (
              copilotModels.length === 0 ? (
                <div style={{ color: "var(--text-muted)", fontSize: 12, textAlign: "center", padding: 8 }}>{t("noRunningInstances")}</div>
              ) : (
                <div style={{ overflowX: "auto" }}>
                  <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
                    <thead><tr>
                      <th style={thStyle}>ID</th>
                      <th style={thStyle}>Owner</th>
                      <th style={thStyle}>{t("modelStatus")}</th>
                      <th style={{ ...thStyle, textAlign: "right" }} />
                    </tr></thead>
                    <tbody>
                      {copilotModels.map((m) => (
                        <tr key={m.id} style={{ borderBottom: "1px solid var(--border)" }}>
                          <td style={{ padding: "6px 10px", fontSize: 12, fontFamily: "monospace" }}>{m.id}</td>
                          <td style={{ padding: "6px 10px", fontSize: 12, color: "var(--text-muted)" }}>{m.ownedBy}</td>
                          <td style={{ padding: "6px 10px", fontSize: 12 }}>
                            {m.mapped ? (
                              <span style={{ color: "var(--green)" }}>{t("mapped")} → <span style={{ fontFamily: "monospace" }}>{m.displayId}</span></span>
                            ) : (
                              <span style={{ color: "var(--yellow, #e5a00d)" }}>{t("unmapped")}</span>
                            )}
                          </td>
                          <td style={{ padding: "6px 10px", textAlign: "right" }}>
                            {!m.mapped && !mappings.some(mm => mm.copilotId === m.id) && (
                              <button onClick={() => quickAddModel(m)} style={{ fontSize: 11, padding: "2px 8px" }}>{t("quickAdd")}</button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )
            )}
          </div>

          {/* Mapping Editor */}
          {mappings.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 16, textAlign: "center" }}>
              {t("noMappings")}<br /><span style={{ fontSize: 12 }}>{t("noMappingsHint")}</span>
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 40px", gap: 8, fontSize: 12, color: "var(--text-muted)", fontWeight: 600 }}>
                <span>{t("copilotId")}</span><span>{t("displayId")}</span><span>{t("displayName")}</span><span />
              </div>
              {mappings.map((m, idx) => (
                <div key={idx} style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 40px", gap: 8 }}>
                  <input value={m.copilotId} onChange={(e) => updateRow(idx, "copilotId", e.target.value)} placeholder={t("modelMappingPlaceholder")} style={{ fontSize: 12, padding: "4px 8px" }} />
                  <input value={m.displayId} onChange={(e) => updateRow(idx, "displayId", e.target.value)} placeholder={t("displayIdPlaceholder")} style={{ fontSize: 12, padding: "4px 8px" }} />
                  <input value={m.displayName ?? ""} onChange={(e) => updateRow(idx, "displayName", e.target.value)} placeholder={t("displayNamePlaceholder")} style={{ fontSize: 12, padding: "4px 8px" }} />
                  <button className="danger" onClick={() => removeRow(idx)} style={{ padding: "2px 6px", fontSize: 11 }}>×</button>
                </div>
              ))}
            </div>
          )}
          <div style={{ display: "flex", gap: 8, marginTop: 12, justifyContent: "flex-end" }}>
            <button onClick={addRow} style={{ fontSize: 13 }}>{t("addMapping")}</button>
            <button className="primary" onClick={() => void saveMappings()} disabled={saving} style={{ fontSize: 13 }}>{saving ? t("saving") : t("save")}</button>
          </div>
        </div>
      )}
    </div>
  )
}

function Dashboard() {
  const [accounts, setAccounts] = useState<Array<Account>>([])
  const [showForm, setShowForm] = useState(false)
  const [loading, setLoading] = useState(true)
  const [proxyPort, setProxyPort] = useState(4141)
  const [pools, setPools] = useState<Array<Pool>>([])
  const [proxySettings, setProxySettings] = useState<ProxySettings>({ proxyURL: "" })
  const t = useT()

  const refresh = useCallback(async () => {
    try { const data = await api.getAccounts(); setAccounts(data) }
    catch (err) { console.error("Failed to fetch accounts:", err) }
    finally { setLoading(false) }
  }, [])

  useEffect(() => {
    void api.getConfig().then((cfg) => setProxyPort(cfg.proxyPort))
    void api.getPools().then(setPools).catch(() => {})
    void api.getProxySettings().then(setProxySettings).catch(() => {})
    void refresh()
    const interval = setInterval(() => void refresh(), 5000)
    return () => clearInterval(interval)
  }, [refresh])

  const handleAdd = async () => { setShowForm(false); await refresh() }
  const handleLogout = () => { setSessionToken(""); window.location.reload() }

  return (
    <div style={{ maxWidth: 960, margin: "0 auto", padding: "24px 16px" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 600 }}>{t("consoleTitle")}</h1>
          <p style={{ color: "var(--text-muted)", fontSize: 14 }}>{t("dashboardSubtitle")}</p>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <LanguageSwitcher />
          <button className="primary" onClick={() => setShowForm(!showForm)}>{showForm ? t("cancel") : t("addAccount")}</button>
          <button onClick={handleLogout}>{t("logout")}</button>
        </div>
      </header>
      <ProxySettingsPanel settings={proxySettings} onChange={setProxySettings} />
      <PoolManagement accounts={accounts} proxyPort={proxyPort} />
      <BatchUsagePanel />
      <ProxyUsagePanel accounts={accounts} />
      <ModelMappingPanel />
      {showForm && <AddAccountForm onComplete={handleAdd} onCancel={() => setShowForm(false)} />}
      {loading
        ? <p style={{ color: "var(--text-muted)", textAlign: "center", padding: 40 }}>{t("loading")}</p>
        : <AccountList accounts={accounts} proxyPort={proxyPort} pools={pools} onRefresh={refresh} />}
    </div>
  )
}

export function App() {
  const [authState, setAuthState] = useState<AuthState>("loading")
  const t = useT()

  useEffect(() => {
    void (async () => {
      try {
        const config = await api.getConfig()
        if (config.needsSetup) { setAuthState("setup"); return }
        const token = getSessionToken()
        if (token) {
          try { await api.checkAuth(); setAuthState("authed"); return } catch { setSessionToken("") }
        }
        setAuthState("login")
      } catch { setAuthState("login") }
    })()
  }, [])

  if (authState === "loading") return <div style={{ color: "var(--text-muted)", textAlign: "center", padding: 120 }}>{t("loading")}</div>
  if (authState === "setup") return <SetupForm onComplete={() => setAuthState("authed")} />
  if (authState === "login") return <LoginForm onLogin={() => setAuthState("authed")} />
  return <Dashboard />
}
