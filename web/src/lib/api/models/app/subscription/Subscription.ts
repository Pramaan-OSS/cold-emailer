export default interface Subscription {
    id: string
    billing_customer_id: string
    billing_subscription_id?: string | null
    // The Billing price being charged; equal to plan.billing_price_id_yearly on a
    // yearly subscription.
    billing_price_id?: string | null
    plan_id: string
    // Nested plan summary as the API returns it (the old flat plan_name field
    // never existed on the wire).
    plan?: {
        id?: string
        name: string
        price?: number
        price_yearly?: number | null
        billing_price_id_yearly?: string | null
    } | null
    status: 'active' | 'canceled' | 'past_due' | 'trialing' | 'incomplete' | 'incomplete_expired' | 'unpaid' | 'paused'
    // True when an operator granted this plan rather than Billing. Such a
    // subscription keeps whatever `status` it had, so this is the only signal
    // that the workspace is entitled. Expiry is resolved server-side.
    managed?: boolean
    current_period_start: Date
    current_period_end: Date
    cancel_at_period_end: boolean
}
