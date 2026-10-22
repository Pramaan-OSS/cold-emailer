import { Logo } from "@/components/svg";
export type AgentMarkState="idle"|"thinking"|"attention";
type Props={size?:number;state?:AgentMarkState;tone?:"color"|"muted";listen?:boolean;celebrate?:boolean;variant?:"blob"|"bare";className?:string};
export default function AgentMark({size=20,className}:Props){return <Logo width={size} height={size} className={className} />;}
