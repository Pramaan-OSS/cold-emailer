import type {ReactNode} from "react";
// Local records stay in the workspace; connected actions are selected in Connections.
export function CrmHeaderStatus(_props: {syncedAt?: Date|string;className?:string}) {return null;}
export function CrmOwnerMappingLink(_props: {onNavigate?:()=>void;className?:string}) {return null;}
export function CrmActionNote(_props: {children:ReactNode;className?:string}) {return null;}
export function CrmUpsertNote(_props: {action:string}) {return null;}
export function CrmDealNote(_props: {creates?:boolean;className?:string}) {return null;}
export function CrmTaskNote(_props: {className?:string}) {return null;}
