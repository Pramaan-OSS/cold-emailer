import {Plug} from "lucide-react";
export default function AppGlyph({name, size=5}: {provider?:string;name?:string;size?:number}) {return <Plug aria-label={name ?? "Connected app"} style={{width:size*4,height:size*4}}/>;}
