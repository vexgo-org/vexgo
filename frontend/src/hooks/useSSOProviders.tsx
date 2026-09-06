import { useEffect, useState } from "react";
import { getVexGoAPI } from "@/api/generated/endpoints";

export type SSOProvider = "github" | "google" | "oidc";

interface UseSSOProvidersResult {
  providers: SSOProvider[];
  allowLocalLogin: boolean;
  loading: boolean;
}

export function useSSOProviders(): UseSSOProvidersResult {
  const [providers, setProviders] = useState<SSOProvider[]>([]);
  const [allowLocalLogin, setAllowLocalLogin] = useState(true);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getVexGoAPI()
      .getSsoProviders()
      .then((r) => r.data)
      .then((data) => {
        setProviders((data.providers as SSOProvider[]) ?? []);
        setAllowLocalLogin(data.allow_local_login ?? true);
      })
      .catch(() => {
        setProviders([]);
        setAllowLocalLogin(true);
      })
      .finally(() => setLoading(false));
  }, []);

  return { providers, allowLocalLogin, loading };
}
