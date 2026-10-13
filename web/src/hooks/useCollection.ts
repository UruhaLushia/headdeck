import type { Collection, Diff } from "../lib/types";
import { useLive } from "./useLive";
import { useResource } from "./useResource";

export function useCollection<T extends { id: string }>(kind: "nodes" | "users") {
  const resource = useResource<Collection<T>>(`/api/${kind}`);
  useLive((event) => {
    if (event.type !== `${kind}.changed`) return;
    const diff = event.data as Diff<T> | undefined;
    if (!diff || !resource.data || diff.base_revision !== resource.data.revision) {
      if (!diff || !resource.data || diff.revision > resource.data.revision)
        void resource.refresh();
      return;
    }
    if (diff.revision <= resource.data.revision) return;
    resource.setData((current) => {
      if (!current || current.revision !== diff.base_revision) return current;
      const items = (current[kind] ?? []) as T[];
      const changed = new Map((diff.upsert ?? []).map((item) => [item.id, item]));
      const removed = new Set(diff.removed ?? []);
      const next = items
        .filter((item) => !removed.has(item.id))
        .map((item) => {
          const value = changed.get(item.id) ?? item;
          changed.delete(item.id);
          return value;
        });
      for (const [id, item] of changed) if (!removed.has(id)) next.push(item);
      return { ...current, [kind]: next, revision: diff.revision };
    });
  });
  return { ...resource, items: (resource.data?.[kind] ?? []) as T[] };
}
