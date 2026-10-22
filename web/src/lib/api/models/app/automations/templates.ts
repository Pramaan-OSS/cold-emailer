// Prebuilt automation starting points. Each is a full graph the "New automation"
// menu / empty state can instantiate. They lean on native (ColdEmail built-in)
// actions so they work without any external connection — the user just fills in
// the specifics (which tag, which pipeline) the template can't know.

import type { AutomationGraph } from "@/lib/api/models/app/automations/Automation";

export interface AutomationTemplate {
    id: string;
    name: string;
    description: string;
    trigger_event: string;
    graph: AutomationGraph;
}

export const AUTOMATION_TEMPLATES: AutomationTemplate[] = [
    {
        id: "tag-hot-replies",
        name: "Tag hot replies",
        description: "When a prospect replies positively, add a tag to the contact.",
        trigger_event: "campaign.reply_received",
        graph: {
            nodes: [
                { id: "trigger", type: "trigger", x: 0, y: 0 },
                { id: "c1", type: "condition", x: 0, y: 150, condition: { field: "field", key: "intent", operator: "equals", value: "positive" } },
                { id: "a1", type: "action", x: 0, y: 300, action: "coldemail.add_tag", config: {} },
            ],
            edges: [
                { id: "e1", source: "trigger", target: "c1", when: "" },
                { id: "e2", source: "c1", target: "a1", when: "true" },
            ],
        },
    },
    {
        id: "deal-on-meeting",
        name: "Deal on meeting booked",
        description: "When a meeting is booked, open a CRM deal for the contact.",
        trigger_event: "meeting.booked",
        graph: {
            nodes: [
                { id: "trigger", type: "trigger", x: 0, y: 0 },
                { id: "a1", type: "action", x: 0, y: 150, action: "coldemail.create_deal", config: { deal_name: "{{.invitee_name}} - {{.event_name}}" } },
            ],
            edges: [{ id: "e1", source: "trigger", target: "a1", when: "" }],
        },
    },
    {
        id: "unsubscribe-on-bounce",
        name: "Unsubscribe on bounce",
        description: "When an email hard-bounces, unsubscribe the contact to protect deliverability.",
        trigger_event: "campaign.email_bounced",
        graph: {
            nodes: [
                { id: "trigger", type: "trigger", x: 0, y: 0 },
                { id: "a1", type: "action", x: 0, y: 150, action: "coldemail.unsubscribe", config: {} },
            ],
            edges: [{ id: "e1", source: "trigger", target: "a1", when: "" }],
        },
    },
    {
        id: "chat-positive-reply",
        name: "Chat on positive reply",
        description: "Ping a Chat channel when a prospect replies positively. (Select a connection, inspect its action and fill its fields.)",
        trigger_event: "campaign.reply_received",
        graph: {
            nodes: [
                { id: "trigger", type: "trigger", x: 0, y: 0 },
                { id: "c1", type: "condition", x: 0, y: 150, condition: { field: "field", key: "intent", operator: "equals", value: "positive" } },
                { id: "a1", type: "action", x: 0, y: 300, action: "connected.execute", config: { pramaan_input: { text: "Positive reply from {{.contact_email}}" } } },
            ],
            edges: [
                { id: "e1", source: "trigger", target: "c1", when: "" },
                { id: "e2", source: "c1", target: "a1", when: "true" },
            ],
        },
    },
];
