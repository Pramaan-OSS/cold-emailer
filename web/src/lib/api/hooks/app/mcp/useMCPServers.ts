import {useAppStore} from "@/stores";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
    listMCPServers,
    createMCPServer,
    updateMCPServer,
    deleteMCPServer,
    refreshMCPServer,
} from "@/lib/api/client/app/mcp/mcp";
import type {
    CreateMCPServer,
    UpdateMCPServer,
} from "@/lib/api/models/app/mcp/MCPServer";

const KEY = ["ai", "connections"];

export function useMCPServers() {
    const scope=useAppStore(s=>`${s.currentOrganization?.id}:${s.user?.id}`);
 return useQuery({
        retry: false,
        queryKey: [...KEY,scope],
        queryFn: () => listMCPServers(),
        staleTime: 30_000,
    });
}

export function useCreateMCPServer() {
    const qc = useQueryClient();
    return useMutation({
        retry:false,
        mutationFn: (data: CreateMCPServer) => createMCPServer(data),
        onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
    });
}

export function useUpdateMCPServer() {
    const qc = useQueryClient();
    return useMutation({
        retry:false,
        mutationFn: (input: { id: string; data: UpdateMCPServer }) =>
            updateMCPServer(input.id, input.data),
        onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
    });
}

export function useDeleteMCPServer() {
    const qc = useQueryClient();
    return useMutation({
        retry:false,
        mutationFn: (id: string) => deleteMCPServer(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
    });
}

export function useRefreshMCPServer() {
    const qc = useQueryClient();
    return useMutation({
        retry:false,
        mutationFn: (id: string) => refreshMCPServer(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
    });
}
