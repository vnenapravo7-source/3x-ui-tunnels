import { z } from 'zod';

export const SidecarTunnelClientSchema = z.object({
  password: z.string().min(1),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z
    .union([z.number(), z.string()])
    .transform((v) => Number(v) || 0)
    .default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type SidecarTunnelClient = z.infer<typeof SidecarTunnelClientSchema>;

export const OpenFluxTransportSchema = z.object({
  type: z.enum(['direct', 'cupsonline', 'yandex', 'vyandex', 'boards', 'mailru']),
  url: z.string().default(''),
  priority: z.number().int().min(1).max(1000).default(100),
  dial: z.string().optional(),
  name: z.string().optional(),
});

export const OpenFluxInboundSettingsSchema = z.object({
  codec: z.enum(['batched', 'legacy']).default('batched'),
  mode: z.enum(['l3', 'l4']).default('l4'),
  negotiate: z.boolean().default(true),
  sessionContextUrl: z.string().default(''),
  transports: z.array(OpenFluxTransportSchema).min(1).max(8),
  clients: z.array(SidecarTunnelClientSchema).default([]),
});
export type OpenFluxInboundSettings = z.infer<typeof OpenFluxInboundSettingsSchema>;

export const WdttInboundSettingsSchema = z.object({
  wgPort: z.number().int().min(1).max(65535).default(56001),
  localPort: z.number().int().min(1).max(65535).default(9000),
  hashes: z.array(z.string()).max(6).default([]),
  clients: z.array(SidecarTunnelClientSchema).default([]),
});
export type WdttInboundSettings = z.infer<typeof WdttInboundSettingsSchema>;

export const CsqttInboundSettingsSchema = z.object({
  hashes: z.array(z.string()).max(6).default([]),
  clients: z.array(SidecarTunnelClientSchema).default([]),
});
export type CsqttInboundSettings = z.infer<typeof CsqttInboundSettingsSchema>;
