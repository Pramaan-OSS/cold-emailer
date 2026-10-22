export type Identity={userId:string;email?:string|null;name?:string|null}|null;
export type StepProperties=Record<string,string|number|boolean>;
export function initErrorReporting():void {}
export function captureException(_error:unknown,_properties?:StepProperties):void {}
export function setErrorIdentity(_next:Identity):void {}
export function noteStep(_message:string,_properties?:StepProperties):void {}
