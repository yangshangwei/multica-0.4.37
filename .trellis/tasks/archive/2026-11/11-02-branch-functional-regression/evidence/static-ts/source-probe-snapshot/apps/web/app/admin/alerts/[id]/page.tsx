import { AdminAlertDetailPage } from "@multica/views/admin";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <AdminAlertDetailPage id={id} />;
}
