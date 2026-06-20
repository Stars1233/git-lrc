import { renderIcon } from '../../components/icons.js';
import { dedupeIdentityLines, getDisplayName, getInitials } from '/static/ui-connectors/session-utils.js';

const { html, useEffect, useState } = window.preact;

export function ProfilePage({ session, onReauthenticate, reauthInProgress }) {
  const displayName = getDisplayName(session);
  const initials = getInitials(session);
  const email = (session && session.user_email) || '';
  const orgLabel = (session && session.org_name) || (session && session.org_id ? `Org #${session.org_id}` : 'Organization unavailable');
  const [avatarFailed, setAvatarFailed] = useState(false);
  const avatarURL = session && session.avatar_url ? session.avatar_url : '';
  const heroLines = dedupeIdentityLines([displayName, email, orgLabel]);
  const heroTitle = heroLines[0] || displayName;
  const heroSubtitle = heroLines[1] || '';

  useEffect(() => {
    setAvatarFailed(false);
  }, [avatarURL]);

  return html`
    <div class="single">
      <section class="card profile-hero">
        <div class="profile-avatar-wrap">
          ${avatarURL && !avatarFailed
            ? html`<img class="profile-avatar" src=${avatarURL} alt="${displayName}" onError=${() => setAvatarFailed(true)} />`
            : html`<div class="profile-avatar profile-avatar-fallback">${initials}</div>`}
        </div>
        <div class="profile-main">
          <div class="profile-title">${heroTitle}</div>
          <div class="profile-sub">${heroSubtitle || 'No profile details available'}</div>
          <div class="row">
            <span class="badge">${orgLabel}</span>
            <span class="badge">${session && session.authenticated ? 'Signed in' : 'Signed out'}</span>
          </div>
        </div>
        <div class="profile-actions">
          <button onClick=${onReauthenticate} disabled=${reauthInProgress}>
            ${renderIcon(html, 'reauthenticate', { className: 'btn-icon' })}
            ${reauthInProgress ? 'Reauthenticating...' : 'Re-authenticate'}
          </button>
        </div>
      </section>

      <section class="card">
        <h2>Profile Details</h2>
        <div class="profile-grid">
          <div class="profile-item">
            <div class="profile-label">Name</div>
            <div class="profile-value">${displayName}</div>
          </div>
          <div class="profile-item">
            <div class="profile-label">Email</div>
            <div class="profile-value">${session && session.user_email ? session.user_email : 'Unavailable'}</div>
          </div>
          <div class="profile-item">
            <div class="profile-label">Organization</div>
            <div class="profile-value">${orgLabel}</div>
          </div>
          <div class="profile-item">
            <div class="profile-label">Organization ID</div>
            <div class="profile-value">${session && session.org_id ? session.org_id : 'Unavailable'}</div>
          </div>
          <div class="profile-item">
            <div class="profile-label">API Endpoint</div>
            <div class="profile-value">${session && session.api_url ? session.api_url : 'Unavailable'}</div>
          </div>
          <div class="profile-item">
            <div class="profile-label">Session Status</div>
            <div class="profile-value">${session && session.authenticated ? 'Authenticated' : 'Unauthenticated'}</div>
          </div>
        </div>
      </section>
    </div>
  `;
}
