import { type ReactNode } from 'react'
import { DownloadIcon, InfoCircleIcon, RulesIcon, SettingsIcon } from '../icons'

// Schede delle Impostazioni, nell'ordine in cui compaiono (su desktop anche
// Ctrl+1…4 e Ctrl+←/→).
export type SettingsTab = 'general' | 'download' | 'rules' | 'info'
export const SETTINGS_TABS: { id: SettingsTab; label: string; icon: ReactNode }[] = [
    { id: 'general', label: 'Generale', icon: <SettingsIcon /> },
    { id: 'download', label: 'Download', icon: <DownloadIcon /> },
    { id: 'rules', label: 'Regole', icon: <RulesIcon /> },
    { id: 'info', label: 'Info', icon: <InfoCircleIcon size={16} /> },
]

// SettingsTabsBar: schede delle Impostazioni, tab orizzontali su desktop e
// pillole scorrevoli su Android (mobile.css). Stanno fuori dall'area che
// scorre, ferme sotto l'header: la scrollbar parte sotto di loro. updateDot
// accende il pallino su Info (aggiornamento disponibile).
export function SettingsTabsBar({
    current,
    onSelect,
    updateDot,
}: {
    current: SettingsTab
    onSelect: (tab: SettingsTab) => void
    updateDot: boolean
}) {
    return (
        <div className="settings-tabs-bar">
            <nav className="settings-tabs" aria-label="Sezioni delle impostazioni">
                {SETTINGS_TABS.map((t) => (
                    <button
                        key={t.id}
                        type="button"
                        className={'settings-tab' + (current === t.id ? ' is-active' : '')}
                        aria-current={current === t.id ? 'page' : undefined}
                        onClick={() => onSelect(t.id)}
                    >
                        {t.icon}
                        <span>{t.label}</span>
                        {t.id === 'info' && updateDot && <span className="update-dot" aria-hidden="true" />}
                    </button>
                ))}
            </nav>
        </div>
    )
}
