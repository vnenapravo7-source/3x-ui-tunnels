import { useState } from 'react';
import { Alert, Button, Modal, Tag } from 'antd';
import { CloudDownloadOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';

export interface OpenFluxUpdateInfo {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  installed: boolean;
}

interface Props {
  open: boolean;
  info: OpenFluxUpdateInfo;
  onClose: () => void;
  onCheck: () => Promise<void>;
  onUpdated: (info: OpenFluxUpdateInfo) => void;
}

export default function OpenFluxUpdateModal({ open, info, onClose, onCheck, onUpdated }: Props) {
  const [busy, setBusy] = useState(false);
  const [checkBusy, setCheckBusy] = useState(false);
  const [modal, contextHolder] = Modal.useModal();

  async function check() {
    setCheckBusy(true);
    try {
      await onCheck();
    } finally {
      setCheckBusy(false);
    }
  }

  async function update() {
    setBusy(true);
    try {
      const result = await HttpUtil.post<OpenFluxUpdateInfo>('/panel/api/server/updateOpenFlux');
      if (!result?.success || !result.obj) return;
      onUpdated(result.obj);
      modal.success({
        title: 'OpenFlux обновлён',
        content:
          'Перезапущены только OpenFlux-подключения. Inbound, ключи, cookies и комнаты Cups сохранены.',
      });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {contextHolder}
      <Modal open={open} title="Серверная часть OpenFlux" footer={null} onCancel={onClose}>
        <Alert
          type={info.updateAvailable ? 'warning' : 'success'}
          showIcon
          className="mb-12"
          title={info.updateAvailable ? 'Доступно обновление' : 'Установлена актуальная сборка'}
          description="Обновляется только проверенный OpenFlux sidecar. Настройки подключений и остальные протоколы не изменяются; при ошибке запуска возвращается предыдущий бинарник."
        />
        <div className="version-list">
          <div className="version-list-item">
            <span>Установлено</span>
            <Tag color={info.installed ? 'green' : 'red'}>{info.currentVersion || '—'}</Tag>
          </div>
          <div className="version-list-item">
            <span>Доступно</span>
            <Tag color="purple">{info.latestVersion || '—'}</Tag>
          </div>
        </div>
        <div className="actions-row">
          <Button loading={checkBusy} onClick={() => void check()}>
            Проверить
          </Button>
          <Button
            type="primary"
            icon={<CloudDownloadOutlined />}
            loading={busy}
            disabled={!info.updateAvailable}
            onClick={() => void update()}
          >
            Обновить OpenFlux
          </Button>
        </div>
      </Modal>
    </>
  );
}
