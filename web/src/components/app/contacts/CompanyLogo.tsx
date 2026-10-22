import {useQuery} from "@tanstack/react-query";
import {DashboardImage} from "@/components/ui/dashboard-image";
import {capability} from "@/lib/pramaan";
import type {ReactNode} from "react";
export default function CompanyLogo({domain,className,fallback=null}:{domain:string;className?:string;fallback?:ReactNode}) {
 const logo=useQuery({queryKey:["company-logo",domain],enabled:!!domain,staleTime:30*60*1000,retry:false,queryFn:()=>capability<{file?:{url?:string}}>("companies.logo.get",{domain,size:64,grayscale:true,format:"png",fallback:"error"})});
 return logo.data?.file?.url?<DashboardImage src={logo.data.file.url} alt="" className={className}/>:<>{fallback}</>;
}
