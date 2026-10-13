export interface Provider {
  id: string;
  name: string;
  kind: string;
  enabled: boolean;
  auto_register: boolean;
  issuer: string;
  client_id: string;
  client_secret?: string;
  secret_configured?: boolean;
  auth_url?: string;
  token_url?: string;
  userinfo_url?: string;
  scopes?: string[];
  subject_claim?: string;
  name_claim?: string;
  avatar_claim?: string;
}
