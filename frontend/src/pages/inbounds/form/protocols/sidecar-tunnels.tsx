import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Col, Input, InputNumber, Row, Select, Space, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';

type TransportType = 'direct' | 'cupsonline' | 'yandex' | 'vyandex' | 'boards' | 'mailru';
type OpenFluxTransport = { type: TransportType; url: string; priority: number };

const transportOptions = [
  { value: 'cupsonline', label: 'Cups.online' },
  { value: 'direct', label: 'Direct TCP' },
  { value: 'yandex', label: 'Yandex Docs' },
  { value: 'vyandex', label: 'Yandex Volga' },
  { value: 'boards', label: 'Yandex Boards' },
  { value: 'mailru', label: 'Mail.ru Docs' },
];

export function OpenFluxFields() {
  const { control, setValue } = useFormContext<InboundFormValues>();
  const transports =
    (useWatch({ control, name: 'settings.transports' }) as OpenFluxTransport[] | undefined) ?? [];

  const update = (index: number, patch: Partial<OpenFluxTransport>) => {
    const next = transports.map((item, i) => (i === index ? { ...item, ...patch } : item));
    setValue('settings.transports' as never, next as never, { shouldDirty: true });
  };

  const remove = (index: number) => {
    setValue('settings.transports' as never, transports.filter((_, i) => i !== index) as never, {
      shouldDirty: true,
    });
  };

  return (
    <>
      <Alert
        type="info"
        showIcon
        message="OpenFlux runs as a managed exit process. Cups.online creates rooms automatically."
        style={{ marginBottom: 16 }}
      />
      <Row gutter={12}>
        <Col span={8}>
          <FormField name={['settings', 'mode']} label="Exit mode">
            <Select
              options={[
                { value: 'l4', label: 'L4 (recommended)' },
                { value: 'l3', label: 'L3' },
              ]}
            />
          </FormField>
        </Col>
        <Col span={8}>
          <FormField name={['settings', 'codec']} label="Codec">
            <Select
              options={[
                { value: 'batched', label: 'batched' },
                { value: 'legacy', label: 'legacy' },
              ]}
            />
          </FormField>
        </Col>
        <Col span={8}>
          <FormField name={['settings', 'negotiate']} label="Multi-session" valueProp="checked">
            <Switch />
          </FormField>
        </Col>
      </Row>
      <Space orientation="vertical" style={{ width: '100%' }} size="small">
        {transports.map((item, index) => (
          <Row gutter={8} key={`${index}-${item.type}`} align="middle">
            <Col span={7}>
              <Select
                value={item.type}
                options={transportOptions}
                style={{ width: '100%' }}
                onChange={(type: TransportType) => update(index, { type, url: '' })}
              />
            </Col>
            <Col span={11}>
              <Input
                value={item.url}
                disabled={item.type === 'cupsonline' || item.type === 'direct'}
                placeholder={
                  item.type === 'cupsonline'
                    ? 'Room code is generated automatically'
                    : item.type === 'direct'
                      ? 'Uses the inbound port'
                      : 'Public document URL'
                }
                onChange={(event) => update(index, { url: event.target.value })}
              />
            </Col>
            <Col span={4}>
              <InputNumber
                value={item.priority}
                min={1}
                max={1000}
                style={{ width: '100%' }}
                onChange={(priority) => update(index, { priority: priority ?? 100 })}
              />
            </Col>
            <Col span={2}>
              <Button
                danger
                type="text"
                icon={<DeleteOutlined />}
                disabled={transports.length <= 1}
                onClick={() => remove(index)}
              />
            </Col>
          </Row>
        ))}
        <Button
          icon={<PlusOutlined />}
          disabled={transports.length >= 8}
          onClick={() =>
            setValue(
              'settings.transports' as never,
              [...transports, { type: 'yandex', url: '', priority: 50 }] as never,
              { shouldDirty: true },
            )
          }
        >
          Add transport
        </Button>
      </Space>
    </>
  );
}

export function WdttFields() {
  return (
    <>
      <Alert
        type="info"
        showIcon
        message="WDTT Plus uses this port for DTLS/WRAP. Client passwords are managed through panel clients."
        style={{ marginBottom: 16 }}
      />
      <Row gutter={12}>
        <Col span={12}>
          <FormField name={['settings', 'wgPort']} label="WireGuard port">
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </FormField>
        </Col>
        <Col span={12}>
          <FormField name={['settings', 'localPort']} label="Android local port">
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </FormField>
        </Col>
      </Row>
      <FormField name={['settings', 'hashes']} label="VK call hashes (up to 6)">
        <Select mode="tags" maxCount={6} tokenSeparators={[',', '\n']} />
      </FormField>
    </>
  );
}

export function CsqttFields() {
  return (
    <>
      <Alert
        type="info"
        showIcon
        message="CSQTT uses the inbound port as its UDP peer port. Client passwords are managed through panel clients."
        style={{ marginBottom: 16 }}
      />
      <FormField name={['settings', 'hashes']} label="VK call hashes (up to 6)">
        <Select mode="tags" maxCount={6} tokenSeparators={[',', '\n']} />
      </FormField>
    </>
  );
}
