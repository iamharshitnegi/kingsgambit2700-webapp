// Local demo files stay in this browser. They never use the production object store.
const memory = new Map<string, Blob>();
const databaseName = "kingsgambit.demo.files.v1";
async function database(): Promise<IDBDatabase | null> {
  if (typeof indexedDB === "undefined") return null;
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => request.result.createObjectStore("files");
    request.onerror = () => reject(new Error("Local file storage could not be opened."));
    request.onsuccess = () => resolve(request.result);
  });
}
export async function putDemoFile(id: string, blob: Blob) {
  const db = await database();
  if (!db) { memory.set(id, blob); return; }
  try { await new Promise<void>((resolve, reject) => { const tx = db.transaction("files", "readwrite"); tx.objectStore("files").put(blob, id); tx.oncomplete = () => resolve(); tx.onerror = () => reject(new Error("Your browser could not store this file. Free some storage and retry.")); tx.onabort = tx.onerror; }); }
  finally { db.close(); }
}
export async function getDemoFile(id: string): Promise<Blob | undefined> {
  const db = await database(); if (!db) return memory.get(id);
  try { return await new Promise<Blob | undefined>((resolve, reject) => { const tx = db.transaction("files", "readonly"), request = tx.objectStore("files").get(id); request.onsuccess = () => resolve(request.result as Blob | undefined); request.onerror = () => reject(new Error("The local demo file could not be read.")); }); }
  finally { db.close(); }
}
export function clearDemoFiles() { memory.clear(); if (typeof indexedDB !== "undefined") indexedDB.deleteDatabase(databaseName); }
