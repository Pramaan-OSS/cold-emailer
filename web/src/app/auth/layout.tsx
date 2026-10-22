import React from "react";
import { Navigate, Outlet, useNavigate } from "@tanstack/react-router";
import { APP_URL } from "@/lib/information";
import { Logo } from "@/components/svg";
import getToken from "@/lib/helper/getToken";

export default function AuthLayout({ redirectIfAuthenticated = true }: { redirectIfAuthenticated?: boolean } = {}) {
 const navigate=useNavigate();
 React.useEffect(()=>{const receive=(event:MessageEvent)=>{if(event.origin===APP_URL&&event.data?.type==="auth")navigate({to:"/app/emails"});};window.addEventListener("message",receive);return()=>window.removeEventListener("message",receive);},[navigate]);
 if(redirectIfAuthenticated&&getToken())return <Navigate to="/app/emails" replace />;
 return <div className="ce-auth"><header className="top"><a href="/" className="brand"><Logo className="size-7"/>Cold Email</a></header><main className="ce-auth-form"><Outlet /></main><p className="ce-auth-note">Your outreach, conversations and contacts in one workspace.</p></div>;
}
