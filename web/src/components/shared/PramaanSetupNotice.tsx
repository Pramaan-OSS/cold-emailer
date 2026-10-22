import useAuthConfig from '@/lib/api/hooks/auth/useAuthConfig';

export default function PramaanSetupNotice() {
  const {config} = useAuthConfig();
  return config.pramaan_configured === false ? <p role="status">Add PRAMAAN_API_KEY to .env</p> : null;
}
