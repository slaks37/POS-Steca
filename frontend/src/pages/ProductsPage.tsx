import { useCallback, useEffect, useMemo, useState } from 'react'

import { EmptyState, ErrorAlert, LoadingRows, Modal } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request, uploadFile } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatRupiah } from '../lib/format'
import type { Product } from '../lib/types'

interface FormState {
  name: string
  category: string
  price: string
  stock: string
  sku: string
  image_url: string
  image_id: string
}

const emptyForm: FormState = { name: '', category: '', price: '', stock: '0', sku: '', image_url: '', image_id: '' }

/** ProductsPage adalah modul inventori yang sinkron dengan sheet "Products". */
export function ProductsPage() {
  const { isOwner, tenant } = useAuth()
  const [products, setProducts] = useState<Product[]>([])
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [editing, setEditing] = useState<Product | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const res = await request<Envelope<Product[]>>('/products')
      setProducts(res.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat produk')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return products
    return products.filter((p) => `${p.name} ${p.sku} ${p.category}`.toLowerCase().includes(q))
  }, [products, query])

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(product: Product) {
    setEditing(product)
    setForm({
      name: product.name,
      category: product.category,
      price: String(product.price),
      stock: String(product.stock),
      sku: product.sku,
      image_url: product.image_url,
      image_id: product.image_id,
    })
    setFormError(null)
    setShowForm(true)
  }

  async function handleUpload(file: File) {
    setUploading(true)
    setFormError(null)
    try {
      const res = await uploadFile<Envelope<{ file_id: string; url: string }>>('/product-images', file)
      setForm((current) => ({ ...current, image_url: res.data.url, image_id: res.data.file_id }))
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal mengunggah gambar')
    } finally {
      setUploading(false)
    }
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    const payload = {
      name: form.name,
      category: form.category,
      price: Number(form.price) || 0,
      stock: Number(form.stock) || 0,
      sku: form.sku,
      image_url: form.image_url,
      image_id: form.image_id,
    }
    try {
      if (editing) {
        await request<Envelope<Product>>(`/products/${editing.id}`, { method: 'PUT', body: payload })
      } else {
        await request<Envelope<Product>>('/products', { method: 'POST', body: payload })
      }
      setShowForm(false)
      await load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan produk')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(product: Product) {
    if (!window.confirm(`Hapus produk "${product.name}"? Baris pada Google Sheets ikut terhapus.`)) return
    setError(null)
    try {
      await request<void>(`/products/${product.id}`, { method: 'DELETE' })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menghapus produk')
    }
  }

  return (
    <div className="stack">
      <div className="card card-pad">
        <div className="row">
          <input
            style={{ flex: '2 1 240px' }}
            placeholder="Cari nama, SKU, atau kategori..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            aria-label="Cari produk"
          />
          <div className="spacer" />
          {tenant?.spreadsheet_url ? (
            <a className="btn btn-secondary btn-sm" href={tenant.spreadsheet_url} target="_blank" rel="noreferrer">
              Buka Google Sheets
            </a>
          ) : null}
          {isOwner ? (
            <button type="button" className="btn" onClick={openCreate}>
              + Tambah produk
            </button>
          ) : null}
        </div>
      </div>

      <ErrorAlert message={error} />

      <div className="card">
        <div className="card-header">
          <h3>Katalog produk</h3>
          <span className="badge">{visible.length} produk</span>
        </div>
        {loading ? (
          <LoadingRows rows={5} />
        ) : visible.length === 0 ? (
          <EmptyState
            title="Belum ada produk"
            hint={isOwner ? 'Klik "Tambah produk" untuk mengisi katalog.' : 'Minta pemilik menambahkan produk.'}
          />
        ) : (
          <div className="table-scroll">
            <table className="table">
              <thead>
                <tr>
                  <th>Produk</th>
                  <th>Kategori</th>
                  <th>SKU</th>
                  <th className="table-num">Harga</th>
                  <th className="table-num">Stok</th>
                  {isOwner ? <th /> : null}
                </tr>
              </thead>
              <tbody>
                {visible.map((product) => (
                  <tr key={product.id}>
                    <td>
                      <div className="row" style={{ gap: 10, flexWrap: 'nowrap' }}>
                        {product.image_url ? (
                          <img
                            src={product.image_url}
                            alt={product.name}
                            style={{ width: 42, height: 42, borderRadius: 8, objectFit: 'cover' }}
                          />
                        ) : (
                          <div
                            className="avatar"
                            style={{ width: 42, height: 42, borderRadius: 8 }}
                            aria-hidden
                          >
                            {product.name.slice(0, 2).toUpperCase()}
                          </div>
                        )}
                        <div>
                          <div style={{ fontWeight: 600 }}>{product.name}</div>
                          <div className="tiny muted">{product.id}</div>
                        </div>
                      </div>
                    </td>
                    <td>{product.category || '-'}</td>
                    <td className="small muted">{product.sku || '-'}</td>
                    <td className="table-num">{formatRupiah(product.price)}</td>
                    <td className="table-num">
                      <span className={product.stock <= 5 ? 'badge badge-batal' : 'badge'}>{product.stock}</span>
                    </td>
                    {isOwner ? (
                      <td>
                        <div className="row" style={{ justifyContent: 'flex-end', flexWrap: 'nowrap' }}>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => openEdit(product)}>
                            Ubah
                          </button>
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => void handleDelete(product)}
                          >
                            Hapus
                          </button>
                        </div>
                      </td>
                    ) : null}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showForm ? (
        <Modal
          title={editing ? 'Ubah produk' : 'Tambah produk'}
          onClose={() => setShowForm(false)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => setShowForm(false)}>
                Batal
              </button>
              <button type="submit" form="product-form" className="btn" disabled={saving || uploading}>
                {saving ? 'Menyimpan...' : 'Simpan'}
              </button>
            </>
          }
        >
          <form id="product-form" onSubmit={handleSave}>
            <ErrorAlert message={formError} />
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor="p-name">Nama produk</label>
              <input
                id="p-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </div>
            <div className="field-row">
              <div className="field">
                <label htmlFor="p-category">Kategori</label>
                <input
                  id="p-category"
                  value={form.category}
                  onChange={(e) => setForm({ ...form, category: e.target.value })}
                  placeholder="Makanan / Minuman"
                />
              </div>
              <div className="field">
                <label htmlFor="p-sku">SKU</label>
                <input id="p-sku" value={form.sku} onChange={(e) => setForm({ ...form, sku: e.target.value })} />
              </div>
            </div>
            <div className="field-row">
              <div className="field">
                <label htmlFor="p-price">Harga (Rp)</label>
                <input
                  id="p-price"
                  inputMode="numeric"
                  value={form.price}
                  onChange={(e) => setForm({ ...form, price: e.target.value })}
                  required
                />
              </div>
              <div className="field">
                <label htmlFor="p-stock">Stok</label>
                <input
                  id="p-stock"
                  inputMode="numeric"
                  value={form.stock}
                  onChange={(e) => setForm({ ...form, stock: e.target.value })}
                  required
                />
              </div>
            </div>
            <div className="field">
              <label htmlFor="p-image">Foto produk</label>
              <input
                id="p-image"
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (file) void handleUpload(file)
                }}
              />
              <div className="tiny muted" style={{ marginTop: 6 }}>
                Gambar diunggah ke folder Google Drive bisnis Anda. Maksimal 5 MB.
              </div>
              {uploading ? <div className="small muted">Mengunggah...</div> : null}
              {form.image_url ? (
                <img
                  src={form.image_url}
                  alt="Pratinjau produk"
                  style={{ marginTop: 10, width: 120, height: 90, objectFit: 'cover', borderRadius: 8 }}
                />
              ) : null}
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  )
}
