import { type CrmInfo, crmInfo } from "@/components/app/crm/crmProviders";
import type { CRMProviderName, CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";

export interface CrmProviderState {
    provider: CRMProviderName;
    // True while the workspace runs its CRM on a connected one (connected, connected).
    isExternal: boolean;
    // The connected CRM's name, logo and words; connected's while none is active.
    crm: CrmInfo;
    settings?: CRMSettings;
    // The provider's web app root for this account ("" when not connected).
    appUrl: string;
    // The connection lacks a permission CRM mode needs, or was revoked.
    needsReconnect: boolean;
    loading: boolean;
}

// Which CRM the workspace runs on. Every CRM surface reads this to show the
// connected CRM's records, wording and logo instead of ColdEmail's own.
export default function useCrmProvider(): CrmProviderState {
 return { provider: "native", isExternal: false, crm: crmInfo("native"), appUrl: "", needsReconnect: false, loading: false };
}
