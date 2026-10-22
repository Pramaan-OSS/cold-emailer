export function initProductAnalytics(): void {}
export type Event =
    | "mailbox_connected"
    | "campaign_launched"
    | "product_tour_finished"
    | "product_tour_hosted_plan";
export function capture(_event:Event,_properties?:Record<string,string|number|boolean>):void {}
