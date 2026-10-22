import {useEffect} from "react";
interface Props { visible:boolean; required?:boolean|null; onToken:(token:string)=>void; onError?:(message?:string)=>void; }
export function SecurityNotice({visible,required,onToken,onError}:Props){
 useEffect(()=>{if(!visible)return;if(required===true)onError?.("The hosted security widget is unavailable. Ask the owner to leave captcha off.");else onToken("");},[visible,required,onToken,onError]);return null;
}

export default SecurityNotice;
