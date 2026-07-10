import { Provider } from "../types";
import { ProviderCard } from "./ProviderCard";
import { JsonForm } from "./JsonForm";
import { Card } from "./ui";

type ProviderListProps = {
  providers: Provider[];
  editingId: string | null;
  onEditStart: (id: string) => void;
  onEditCancel: () => void;
  onDelete: (id: string) => void;
  onSaved: () => void;
};

export function ProviderList({
  providers,
  editingId,
  onEditStart,
  onEditCancel,
  onDelete,
  onSaved,
}: ProviderListProps) {
  return (
    <>
      {[...providers]
        .sort((a, b) => a.id.localeCompare(b.id))
        .map((provider) => (
          <Card key={provider.id}>
            {editingId === provider.id ? (
              <JsonForm
                mode="edit"
                id={provider.id}
                onCancel={onEditCancel}
                onSaved={onSaved}
              />
            ) : (
              <ProviderCard
                provider={provider}
                onEdit={onEditStart}
                onDelete={onDelete}
              />
            )}
          </Card>
        ))}
    </>
  );
}
