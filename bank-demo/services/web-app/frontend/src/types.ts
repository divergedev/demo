export interface AccountData {
  accounts: {
    id: string;
    name: string;
    balance: number;
    type: string;
  }[];
  service: string;
  version: string;
}

export interface ModuleInfo {
  url: string;
  version: string;
}

export interface RegistryResponse {
  modules: Record<string, ModuleInfo>;
}

export interface TopologyNode {
  id: string;
  name: string;
  version: string;
  isPreview: boolean;
}
