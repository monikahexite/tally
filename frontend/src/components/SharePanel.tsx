import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { pollUrl } from '../lib/api'

export function SharePanel({ slug, question }: { slug: string; question: string }) {
  const url = pollUrl(slug)
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1800)
    } catch {
      window.prompt('Copy this link', url)
    }
  }

  async function nativeShare() {
    try {
      await navigator.share({ title: question, text: `Vote: ${question}`, url })
    } catch {
      /* user cancelled */
    }
  }

  return (
    <section className="card share">
      <div className="share__body">
        <h3>Share this poll</h3>
        <p className="muted">Anyone with the link can vote. Results update live for everyone.</p>
        <div className="copy-field">
          <input readOnly value={url} onFocus={(e) => e.currentTarget.select()} aria-label="Poll link" />
          <button className="btn btn--primary" onClick={copy}>
            {copied ? 'Copied ✓' : 'Copy link'}
          </button>
        </div>
        <div className="share__actions">
          {'share' in navigator && (
            <button className="btn btn--ghost" onClick={nativeShare}>
              Share…
            </button>
          )}
          <Link className="btn btn--ghost" to={`/p/${slug}/present`}>
            Present mode
          </Link>
        </div>
      </div>
      <div className="qr" aria-label="QR code for the poll link">
        <QRCodeSVG value={url} size={132} marginSize={1} />
      </div>
    </section>
  )
}
