import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { api } from "../lib/api";
import { ResourceCache } from "../lib/resourceCache";

const Context = createContext<ResourceCache | null>(null);
export const createResourceCache = () => new ResourceCache((path, signal) => api.get(path, signal));

export function ResourceProvider({ children }: { children: ReactNode }) {
  const [cache] = useState(createResourceCache);
  useEffect(() => () => cache.cancel(), [cache]);
  return <Context value={cache}>{children}</Context>;
}

export function useResourceCache() {
  return useContext(Context);
}
