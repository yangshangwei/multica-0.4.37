import { AdminUserDetailPage } from "@multica/views/admin";
export default async function UserPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <AdminUserDetailPage id={id} />;
}
