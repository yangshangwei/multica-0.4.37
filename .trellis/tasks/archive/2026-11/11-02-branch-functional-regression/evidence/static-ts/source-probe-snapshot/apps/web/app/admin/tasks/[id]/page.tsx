import { AdminExecutionDetailPage } from "@multica/views/admin";
export default async function Page({ params }: {
  params: Promise<{
    id: string;
  }>;
}) { const { id } = await params; return <AdminExecutionDetailPage id={id}/>; }
