import { useEffect, useState } from "react";
import { sdk } from "@/lib/sdk";

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
    sdk.sso
      .providers()
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
