export function initErrorReporting():void {}
export type Event="form_viewed"|"form_started"|"form_submitted";
export function track(_event:Event,_form:string):void {}
