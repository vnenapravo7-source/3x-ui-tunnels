import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ComponentType, CSSProperties } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Drawer, Layout, Menu, Spin, Tooltip } from 'antd';
import type { MenuProps } from 'antd';
import {
  ApiOutlined,
  ApartmentOutlined,
  CloseOutlined,
  CloudServerOutlined,
  CloudSyncOutlined,
  ClusterOutlined,
  CodeOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DiscordOutlined,
  ExportOutlined,
  GithubOutlined,
  GlobalOutlined,
  ImportOutlined,
  LogoutOutlined,
  MailOutlined,
  MenuOutlined,
  MessageOutlined,
  MoonFilled,
  MoonOutlined,
  PushpinFilled,
  PushpinOutlined,
  ReadOutlined,
  SafetyOutlined,
  SearchOutlined,
  SettingOutlined,
  SunOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { useStatusQuery } from '@/api/queries/useStatusQuery';
import { useCommandPalette } from '@/components/command-palette/useCommandPalette';
import PanelUpdateModal from '@/pages/index/PanelUpdateModal';
import type { PanelUpdateInfo } from '@/pages/index/PanelUpdateModal';
import OpenFluxUpdateModal from '@/pages/index/OpenFluxUpdateModal';
import type { OpenFluxUpdateInfo } from '@/pages/index/OpenFluxUpdateModal';
import VersionModal from '@/pages/index/VersionModal';
import './AppSidebar.css';

// The palette listens for Ctrl as well as Cmd, so the chip must not show a
// Mac glyph to the Linux and Windows operators who are most of this panel's.
const SHORTCUT_MODIFIER = /Mac|iPhone|iPad|iPod/.test(navigator.userAgent) ? '⌘' : 'Ctrl';
const DOCS_URL = 'https://docs.sanaei.dev/';
const LOGOUT_KEY = '__logout__';
const RAIL_WIDTH = 72;
const SIDER_WIDTH = 220;
const SIDEBAR_PINNED_KEY = 'sidebar-pinned';
const UPDATE_CHECKED_KEY = 'fork-release-checked-this-login';
const UPDATE_SKIPPED_KEY = 'fork-release-skipped';

let hoveredAcrossRemounts = false;

type IconName =
  | 'dashboard'
  | 'inbound'
  | 'team'
  | 'groups'
  | 'setting'
  | 'tool'
  | 'cluster'
  | 'hosts'
  | 'logout'
  | 'apidocs'
  | 'outbound'
  | 'routing';

const iconByName: Record<IconName, ComponentType> = {
  dashboard: DashboardOutlined,
  inbound: ImportOutlined,
  team: TeamOutlined,
  groups: TagsOutlined,
  setting: SettingOutlined,
  tool: ToolOutlined,
  cluster: ClusterOutlined,
  hosts: GlobalOutlined,
  logout: LogoutOutlined,
  apidocs: ApiOutlined,
  outbound: ExportOutlined,
  routing: SwapOutlined,
};

function DocsButton({ ariaLabel }: { ariaLabel: string }) {
  return (
    <a
      href={DOCS_URL}
      target="_blank"
      rel="noopener noreferrer"
      className="sidebar-docs"
      aria-label={ariaLabel}
      title={ariaLabel}
    >
      <ReadOutlined />
    </a>
  );
}

function VersionBadge({
  version,
  collapsed,
  onCheck,
}: {
  version: string;
  collapsed?: boolean;
  onCheck: () => void;
}) {
  const label = version || 'fork —';
  return (
    <button
      type="button"
      className="sider-version"
      aria-label={`Проверить обновление форка (${label})`}
      title={`Проверить обновление форка (${label})`}
      onClick={onCheck}
    >
      <GithubOutlined />
      {!collapsed && <span className="sider-version-text">{label}</span>}
    </button>
  );
}

function OpenFluxVersionBadge({
  version,
  collapsed,
  onCheck,
}: {
  version: string;
  collapsed?: boolean;
  onCheck: () => void;
}) {
  const label = version ? `OpenFlux ${version}` : 'OpenFlux —';
  return (
    <button
      type="button"
      className="sider-version"
      aria-label={`Проверить обновление серверной части ${label}`}
      title={`Проверить обновление серверной части ${label}`}
      onClick={onCheck}
    >
      <CloudSyncOutlined />
      {!collapsed && <span className="sider-version-text">{label}</span>}
    </button>
  );
}

function XrayVersionBadge({
  version,
  collapsed,
  onClick,
}: {
  version: string;
  collapsed?: boolean;
  onClick: () => void;
}) {
  const normalized = version.trim().replace(/^v/i, '');
  const label = normalized && normalized !== 'Unknown' ? `Xray v${normalized}` : 'Xray —';
  return (
    <button
      type="button"
      className="sider-version"
      aria-label={`Показать и сменить версию ${label}`}
      title={`Показать и сменить версию ${label}`}
      onClick={onClick}
    >
      <ToolOutlined />
      {!collapsed && <span className="sider-version-text">{label}</span>}
    </button>
  );
}

function ThemeCycleButton({
  id,
  isDark,
  isUltra,
  onCycle,
  ariaLabel,
}: {
  id: string;
  isDark: boolean;
  isUltra: boolean;
  onCycle: () => void;
  ariaLabel: string;
}) {
  const icon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;
  return (
    <button
      id={id}
      type="button"
      className="sidebar-theme-cycle"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onCycle}
    >
      {icon}
    </button>
  );
}

function readSidebarPinned() {
  try {
    return localStorage.getItem(SIDEBAR_PINNED_KEY) === 'true';
  } catch {
    return false;
  }
}

function saveSidebarPinned(pinned: boolean) {
  try {
    localStorage.setItem(SIDEBAR_PINNED_KEY, String(pinned));
  } catch {}
}

export default function AppSidebar() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra } = useTheme();
  const { open: openCommandPalette } = useCommandPalette();
  const navigate = useNavigate();
  const { pathname, hash } = useLocation();
  const { allSetting } = useAllSettings();
  const { status, refresh: refreshStatus } = useStatusQuery();
  const showSubFormats = !!(allSetting.subJsonEnable || allSetting.subClashEnable);
  const showSubBalancers = !!allSetting.subJsonEnable;

  const [hovered, setHovered] = useState(() => hoveredAcrossRemounts);
  const [pinned, setPinned] = useState(readSidebarPinned);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<PanelUpdateInfo>({
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
  });
  const [updateOpen, setUpdateOpen] = useState(false);
  const [autoPrompt, setAutoPrompt] = useState(false);
  const [updateBusy, setUpdateBusy] = useState(false);
  const [openFluxInfo, setOpenFluxInfo] = useState<OpenFluxUpdateInfo>({
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
    installed: false,
  });
  const [openFluxUpdateOpen, setOpenFluxUpdateOpen] = useState(false);
  const [xrayVersionOpen, setXrayVersionOpen] = useState(false);
  const [xrayUpdateBusy, setXrayUpdateBusy] = useState(false);
  const updateCheckStarted = useRef(false);
  const railCollapsed = !hovered && !pinned;
  const railStyle = useMemo(
    () => ({ '--sider-rail': `${pinned ? SIDER_WIDTH : RAIL_WIDTH}px` }) as CSSProperties,
    [pinned],
  );
  const rootRef = useRef<HTMLDivElement>(null);

  const updateHovered = useCallback((value: boolean) => {
    hoveredAcrossRemounts = value;
    setHovered(value);
  }, []);

  const togglePinned = useCallback(() => {
    const next = !pinned;
    saveSidebarPinned(next);
    setPinned(next);
  }, [pinned]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const el = rootRef.current;
      if (el) updateHovered(el.matches(':hover'));
    }, 150);
    return () => window.clearTimeout(timer);
  }, [updateHovered]);

  const checkForkRelease = useCallback(async (manual = false) => {
    try {
      const msg = await HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo');
      if (!msg?.success || !msg.obj) return;
      setUpdateInfo(msg.obj);
      if (manual) {
        setAutoPrompt(false);
        setUpdateOpen(true);
        return;
      }
      let skipped = false;
      try {
        sessionStorage.setItem(UPDATE_CHECKED_KEY, '1');
        skipped = localStorage.getItem(UPDATE_SKIPPED_KEY) === msg.obj.latestVersion;
      } catch {
        /* Storage may be disabled. */
      }
      if (msg.obj.channel === 'fork' && msg.obj.updateAvailable && !skipped) {
        setAutoPrompt(true);
        setUpdateOpen(true);
      }
    } catch {
      /* A transient GitHub/API failure must not break navigation. */
    }
  }, []);

  const checkOpenFluxRelease = useCallback(async (manual = false) => {
    try {
      const msg = await HttpUtil.get<OpenFluxUpdateInfo>(
        '/panel/api/server/getOpenFluxUpdateInfo',
        undefined,
        { silent: !manual },
      );
      if (!msg?.success || !msg.obj) return;
      setOpenFluxInfo(msg.obj);
      if (manual) setOpenFluxUpdateOpen(true);
    } catch {
      /* A transient release check must not break navigation. */
    }
  }, []);

  useEffect(() => {
    if (updateCheckStarted.current) return;
    updateCheckStarted.current = true;
    try {
      if (sessionStorage.getItem(UPDATE_CHECKED_KEY)) return;
    } catch {
      /* Continue without session storage. */
    }
    const timer = window.setTimeout(() => void checkForkRelease(), 0);
    return () => {
      window.clearTimeout(timer);
      updateCheckStarted.current = false;
    };
  }, [checkForkRelease]);

  useEffect(() => {
    const timer = window.setTimeout(() => void checkOpenFluxRelease(), 250);
    return () => window.clearTimeout(timer);
  }, [checkOpenFluxRelease]);

  const currentTheme: 'light' | 'dark' = isDark ? 'dark' : 'light';
  const panelVersion = updateInfo.currentVersion;

  const tabs = useMemo<{ key: string; icon: IconName; title: string }[]>(
    () => [
      { key: '/', icon: 'dashboard', title: t('menu.dashboard') },
      { key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') },
      { key: '/clients', icon: 'team', title: t('menu.clients') },
      { key: '/groups', icon: 'groups', title: t('menu.groups') },
      { key: '/nodes', icon: 'cluster', title: t('menu.nodes') },
      { key: '/hosts', icon: 'hosts', title: t('menu.hosts') },
      { key: '/outbound', icon: 'outbound', title: t('menu.outbounds') },
      { key: '/routing', icon: 'routing', title: t('menu.routing') },
      { key: '/settings', icon: 'setting', title: t('menu.settings') },
      { key: '/xray', icon: 'tool', title: t('menu.xray') },
      { key: '/api-docs', icon: 'apidocs', title: t('menu.apiDocs') },
      { key: LOGOUT_KEY, icon: 'logout', title: t('logout') },
    ],
    [t],
  );

  const navItems = useMemo(() => tabs.filter((tab) => tab.icon !== 'logout'), [tabs]);
  const utilItems = useMemo(() => tabs.filter((tab) => tab.icon === 'logout'), [tabs]);

  const settingsChildren = useMemo<NonNullable<MenuProps['items']>>(() => {
    const children: NonNullable<MenuProps['items']> = [
      {
        key: '/settings#general',
        icon: <SettingOutlined />,
        label: t('pages.settings.panelSettings'),
      },
      {
        key: '/settings#security',
        icon: <SafetyOutlined />,
        label: t('pages.settings.securitySettings'),
      },
      {
        key: '/settings#telegram',
        icon: <MessageOutlined />,
        label: t('pages.settings.TGBotSettings'),
      },
      { key: '/settings#email', icon: <MailOutlined />, label: t('pages.settings.emailSettings') },
      {
        key: '/settings#discord',
        icon: <DiscordOutlined />,
        label: t('pages.settings.discordSettings'),
      },
      {
        key: '/settings#subscription',
        icon: <CloudServerOutlined />,
        label: t('pages.settings.subSettings'),
      },
    ];
    if (showSubFormats) {
      children.push({
        key: '/settings#subscription-formats',
        icon: <CodeOutlined />,
        label: t('menu.subFormats'),
      });
    }
    if (showSubBalancers) {
      children.push({
        key: '/settings#subscription-balancers',
        icon: <ApartmentOutlined />,
        label: t('pages.settings.subBalancers.menu'),
      });
    }
    return children;
  }, [t, showSubFormats, showSubBalancers]);

  const xrayChildren = useMemo<NonNullable<MenuProps['items']>>(
    () => [
      { key: '/xray#basic', icon: <SettingOutlined />, label: t('pages.xray.basicTemplate') },
      { key: '/xray#balancer', icon: <ClusterOutlined />, label: t('pages.xray.Balancers') },
      { key: '/xray#dns', icon: <DatabaseOutlined />, label: 'DNS' },
      { key: '/xray#advanced', icon: <CodeOutlined />, label: t('pages.xray.advancedTemplate') },
    ],
    [t],
  );

  const settingsActive = pathname === '/settings';
  const xrayActive = pathname === '/xray';
  const selectedKey = settingsActive
    ? `/settings${hash || '#general'}`
    : xrayActive
      ? `/xray${hash || '#basic'}`
      : pathname === ''
        ? '/'
        : pathname;

  const openSubmenu = settingsActive ? '/settings' : xrayActive ? '/xray' : null;
  const [openKeys, setOpenKeys] = useState<string[]>(() => (openSubmenu ? [openSubmenu] : []));
  if (openSubmenu && !openKeys.includes(openSubmenu)) {
    setOpenKeys([...openKeys, openSubmenu]);
  }

  const toMenuItems = useCallback(
    (items: typeof tabs): MenuProps['items'] =>
      items.map((tab) => {
        const Icon = iconByName[tab.icon];
        if (tab.key === '/settings') {
          return { key: tab.key, icon: <Icon />, label: tab.title, children: settingsChildren };
        }
        if (tab.key === '/xray') {
          return { key: tab.key, icon: <Icon />, label: tab.title, children: xrayChildren };
        }
        return { key: tab.key, icon: <Icon />, label: tab.title, title: '' };
      }),
    [settingsChildren, xrayChildren],
  );

  const openLink = useCallback(
    async (key: string) => {
      if (key === LOGOUT_KEY) {
        try {
          sessionStorage.removeItem(UPDATE_CHECKED_KEY);
        } catch {}
        await HttpUtil.post('/logout');
        window.location.href = window.X_UI_BASE_PATH || '/';
        return;
      }
      navigate(key);
    },
    [navigate],
  );

  const onMenuClick = useCallback<NonNullable<MenuProps['onClick']>>(
    ({ key }) => {
      openLink(String(key));
    },
    [openLink],
  );

  const cycleTheme = useCallback(
    (id: string) => {
      pauseAnimationsUntilLeave(id);
      if (!isDark) {
        toggleTheme();
        if (isUltra) toggleUltra();
      } else if (!isUltra) {
        toggleUltra();
      } else {
        toggleUltra();
        toggleTheme();
      }
    },
    [isDark, isUltra, toggleTheme, toggleUltra],
  );

  return (
    <div
      ref={rootRef}
      className={`ant-sidebar${pinned ? ' sidebar-pinned' : ''}`}
      style={railStyle}
      onMouseEnter={() => updateHovered(true)}
      onMouseLeave={() => updateHovered(false)}
    >
      <Layout.Sider
        theme={currentTheme}
        width={SIDER_WIDTH}
        collapsedWidth={RAIL_WIDTH}
        collapsed={railCollapsed}
      >
        <div className="sider-brand">
          <div className="brand-block">
            <span className="brand-text">{railCollapsed ? '3X' : '3X-UI'}</span>
          </div>
          {!railCollapsed && (
            <div className="brand-actions">
              <button
                type="button"
                className="sidebar-pin"
                aria-label={t('menu.pinSidebar')}
                aria-pressed={pinned}
                title={t(pinned ? 'menu.unpinSidebar' : 'menu.pinSidebar')}
                onClick={togglePinned}
              >
                {pinned ? <PushpinFilled /> : <PushpinOutlined />}
              </button>
              <DocsButton ariaLabel={t('menu.docs') || 'Documentation'} />
              <ThemeCycleButton
                id="theme-cycle"
                isDark={isDark}
                isUltra={isUltra}
                onCycle={() => cycleTheme('theme-cycle')}
                ariaLabel={t('menu.theme')}
              />
            </div>
          )}
        </div>
        <Tooltip
          title={
            railCollapsed ? t('commandPalette.title') || 'Command Palette (Ctrl + K)' : undefined
          }
          placement="right"
        >
          <button
            type="button"
            className={`sidebar-command-trigger${railCollapsed ? ' collapsed' : ''}`}
            onClick={openCommandPalette}
            aria-label={t('commandPalette.title') || 'Command Palette (Ctrl + K)'}
          >
            <span className="sidebar-command-left">
              <SearchOutlined className="sidebar-command-icon" />
              <span className="sidebar-command-text">
                {t('commandPalette.search') || 'Search...'}
              </span>
            </span>
            <span className="sidebar-command-kbd">
              <span className="kbd-cmd">{SHORTCUT_MODIFIER}</span>
              <span className="kbd-key">K</span>
            </span>
          </button>
        </Tooltip>
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          openKeys={railCollapsed ? undefined : openKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          className="sider-nav"
          items={toMenuItems(navItems)}
          onClick={onMenuClick}
        />
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          className="sider-utility"
          items={toMenuItems(utilItems)}
          onClick={onMenuClick}
        />
        <div className="sider-footer">
          <XrayVersionBadge
            version={status.xray.version}
            collapsed={railCollapsed}
            onClick={() => setXrayVersionOpen(true)}
          />
          <OpenFluxVersionBadge
            version={openFluxInfo.currentVersion}
            collapsed={railCollapsed}
            onCheck={() => void checkOpenFluxRelease(true)}
          />
          <VersionBadge
            version={panelVersion}
            collapsed={railCollapsed}
            onCheck={() => void checkForkRelease(true)}
          />
        </div>
      </Layout.Sider>

      <Drawer
        placement="left"
        closable={false}
        open={drawerOpen}
        rootClassName={currentTheme}
        size="min(82vw, 320px)"
        styles={{
          wrapper: { padding: 0 },
          body: { padding: 0, display: 'flex', flexDirection: 'column', height: '100%' },
          header: { display: 'none' },
        }}
        onClose={() => setDrawerOpen(false)}
      >
        <div className="drawer-header">
          <div className="brand-block">
            <span className="drawer-brand">3X-UI</span>
          </div>
          <div className="drawer-header-actions">
            <DocsButton ariaLabel={t('menu.docs') || 'Documentation'} />
            <ThemeCycleButton
              id="theme-cycle-drawer"
              isDark={isDark}
              isUltra={isUltra}
              onCycle={() => cycleTheme('theme-cycle-drawer')}
              ariaLabel={t('menu.theme')}
            />
            <button
              className="drawer-close"
              type="button"
              aria-label={t('close')}
              onClick={() => setDrawerOpen(false)}
            >
              <CloseOutlined />
            </button>
          </div>
        </div>
        <button
          type="button"
          className="sidebar-command-trigger"
          onClick={() => {
            setDrawerOpen(false);
            openCommandPalette();
          }}
          aria-label={t('commandPalette.title') || 'Command Palette (Ctrl + K)'}
          style={{ margin: '8px 12px 4px', width: 'calc(100% - 24px)' }}
        >
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <SearchOutlined className="sidebar-command-icon" />
            <span>{t('commandPalette.search') || 'Search...'}</span>
          </span>
          <span className="sidebar-command-kbd">
            <span className="kbd-cmd">{SHORTCUT_MODIFIER}</span>
            <span className="kbd-key">K</span>
          </span>
        </button>
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          openKeys={openKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          className="drawer-menu drawer-nav"
          items={toMenuItems(navItems)}
          onClick={(info) => {
            onMenuClick(info);
            setDrawerOpen(false);
          }}
        />
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          className="drawer-menu drawer-utility"
          items={toMenuItems(utilItems)}
          onClick={(info) => {
            onMenuClick(info);
            setDrawerOpen(false);
          }}
        />
        <div className="drawer-footer">
          <XrayVersionBadge
            version={status.xray.version}
            onClick={() => setXrayVersionOpen(true)}
          />
          <OpenFluxVersionBadge
            version={openFluxInfo.currentVersion}
            onCheck={() => void checkOpenFluxRelease(true)}
          />
          <VersionBadge version={panelVersion} onCheck={() => void checkForkRelease(true)} />
        </div>
      </Drawer>

      {!drawerOpen && (
        <button
          className="drawer-handle"
          type="button"
          aria-label={t('menu.openMenu')}
          onClick={() => setDrawerOpen(true)}
        >
          <MenuOutlined />
        </button>
      )}
      <PanelUpdateModal
        open={updateOpen}
        info={updateInfo}
        onCheck={() => checkForkRelease(true)}
        onClose={() => setUpdateOpen(false)}
        onBusy={({ busy }) => setUpdateBusy(busy)}
        autoPrompt={autoPrompt}
        onLater={() => setUpdateOpen(false)}
        onSkip={() => {
          try {
            localStorage.setItem(UPDATE_SKIPPED_KEY, updateInfo.latestVersion);
          } catch {}
          setUpdateOpen(false);
        }}
      />
      <OpenFluxUpdateModal
        open={openFluxUpdateOpen}
        info={openFluxInfo}
        onClose={() => setOpenFluxUpdateOpen(false)}
        onCheck={() => checkOpenFluxRelease(true)}
        onUpdated={(info) => setOpenFluxInfo(info)}
      />
      <VersionModal
        open={xrayVersionOpen}
        status={status}
        onClose={() => setXrayVersionOpen(false)}
        onBusy={({ busy }) => setXrayUpdateBusy(busy)}
        onUpdated={refreshStatus}
      />
      <Spin
        spinning={updateBusy || xrayUpdateBusy}
        fullscreen
        tip={xrayUpdateBusy ? 'Переключение Xray…' : 'Обновление панели…'}
      />
    </div>
  );
}
