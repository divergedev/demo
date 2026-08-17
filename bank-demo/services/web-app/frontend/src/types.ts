export interface AccountData {
  balance: number;
}

export interface ModuleInfo {
  name: string;
  url: string;
}

export interface RegistryResponse {
  modules: ModuleInfo[];
  preview_id: string;
}

export interface TopologyNode {
  id: string;
  name: string;
  version: string;
  isPreview: boolean;
}
