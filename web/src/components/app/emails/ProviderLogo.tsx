import {Logo} from "@/components/svg";
export default function ProviderLogo({className,size="md"}:{id?:string|null;size?:"xs"|"sm"|"md"|"lg"|"xl";framed?:boolean;muted?:boolean;title?:string;className?:string}) {const sizes={xs:16,sm:20,md:24,lg:28,xl:36};return <Logo width={sizes[size]} height={sizes[size]} className={className} aria-label="Pramaan mailbox"/>;}

export function LogoStack({className,size="md"}:{ids?:string[];max?:number;size?:"xs"|"sm"|"md"|"lg"|"xl";className?:string}) {return <ProviderLogo size={size} className={className}/>;}
