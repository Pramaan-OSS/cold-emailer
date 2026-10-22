import type {
    MCPServer,
    CreateMCPServer,
    UpdateMCPServer,
} from "@/lib/api/models/app/mcp/MCPServer";
import Request from "../../Request";

export async function listMCPServers(): Promise<{ data: MCPServer[] }> {
    return await Request<{ data: MCPServer[] }>({
        method: "GET",
        url: `/ai/connections`,
        authorization: true,
    });
}

export async function createMCPServer(data: CreateMCPServer): Promise<MCPServer> {
    return await Request<MCPServer>({
        method: "POST",
        url: `/ai/connections`,
        data,
        headers: { "Idempotency-Key": crypto.randomUUID() },
        authorization: true,
    });
}

export async function updateMCPServer(id: string, data: UpdateMCPServer): Promise<MCPServer> {
    return await Request<MCPServer>({
        method: "PATCH",
        url: `/ai/connections/${id}`,
        data,
        authorization: true,
    });
}

export async function deleteMCPServer(id: string): Promise<{ deleted: boolean }> {
    return await Request<{ deleted: boolean }>({
        method: "DELETE",
        url: `/ai/connections/${id}`,
        authorization: true,
    });
}

export async function refreshMCPServer(id: string): Promise<MCPServer> {
    return await Request<MCPServer>({
        method: "POST",
        url: `/ai/connections/${id}/refresh`,
        authorization: true,
    });
}

export async function callMCPTool(id:string, toolRef:string, input:Record<string,unknown>, key:string):Promise<import('@/lib/api/models/app/mcp/MCPServer').RemoteToolResult> {
 return Request({method:'POST',url:`/ai/connections/${id}/call`,data:{tool_ref:toolRef,input,approved:true},headers:{'Idempotency-Key':key},authorization:true});
}
