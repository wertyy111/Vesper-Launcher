import { photinoBridge } from '../../bridge';
import type { PanelRenderProps } from '../../types';
import { AvatarImage } from '../common/AvatarImage';

export function FriendsPanel({ launcher, friendDraft, setFriendDraft, setFriendDirty }: PanelRenderProps) {
  const friends = launcher.friends;
  const friendItems = (friends.friends ?? []) as Array<Record<string, any>>;
  const incomingRequests = (friends.incomingRequests ?? []) as Array<Record<string, any>>;
  const friendSummary = `Друзей: ${friendItems.length} · входящих: ${incomingRequests.length} · исходящих: ${friends.outgoingRequestCount ?? 0}`;

  return (
    <div className="friends-wpf-panel">
      <h2 className="friends-title">Друзья</h2>

      <section className="friends-profile-card">
        <div className="friends-profile-avatar">
          {friends.profileAvatarUrl ? <img src={friends.profileAvatarUrl} alt="profile" /> : <span>{friends.profileAvatarPlaceholder}</span>}
        </div>

        <div className="friends-profile-info">
          <span>Мой профиль</span>
          <strong>Ник: {friends.profileNickname || '-'}</strong>
          <p>{friends.profileType || 'Тип входа: оффлайн'}</p>
        </div>

        <div className="friends-inline-add">
          <strong>Добавить друга</strong>
          <p>Введи ник и отправь заявку.</p>
          <div className="friends-add-row">
            <input
              className="launcher-input"
              value={friendDraft}
              onChange={(event) => {
                setFriendDirty(true);
                setFriendDraft(event.target.value);
                photinoBridge.sendCommand('friends.setNickname', { value: event.target.value });
              }}
              placeholder="Введите ник"
            />
            <button
              className="friends-add-button"
              disabled={!friends.canManage}
              onClick={() => {
                setFriendDirty(false);
                photinoBridge.sendCommand('friends.add');
              }}
              title="Отправить заявку"
              type="button"
            >
              +
            </button>
          </div>
        </div>
      </section>

      <section className="friends-list-section">
        <h3>Друзья</h3>
        <p className="friends-cloud-status">{friends.cloudStatus || friendSummary}</p>
        <p className="friends-vesper-status">{friends.vesperNetStatus}</p>

        <div className="friends-list-shell">
          <div className="friends-list wpf-friends-list">
            {friendItems.length === 0 ? <p className="friends-empty-copy">Список друзей пуст.</p> : null}
            {friendItems.map((friend) => (
              <article key={String(friend.username)} className="wpf-friend-card">
                <div className="wpf-friend-avatar">
                  <AvatarImage url={friend.avatarUrl} placeholder={friend.avatarPlaceholder} alt={String(friend.username)} />
                </div>

                <div className="wpf-friend-info">
                  <strong>{friend.username}</strong>
                  <p><span className={`presence-dot ${friend.isOnline ? 'online' : ''}`} />{friend.presenceText}</p>
                  {friend.activityText ? <span>{friend.activityText}</span> : null}
                  {friend.versionText ? <span className="friend-version">{friend.versionText}</span> : null}
                  {friend.joinAddressText ? <span className="friend-address">{friend.joinAddressText}</span> : null}
                </div>

                <div className="wpf-friend-actions">
                  {friend.canConnect ? (
                    <button className="subtle-button compact" onClick={() => photinoBridge.sendCommand('friends.connect', { username: friend.username })} type="button">Подключиться</button>
                  ) : null}
                  <button className="danger-button compact" disabled={!friends.canManage} onClick={() => photinoBridge.sendCommand('friends.remove', { username: friend.username })} type="button">Удалить</button>
                </div>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className={`incoming-friends-section ${incomingRequests.length === 0 ? 'empty' : ''}`}>
        <h3>Входящие заявки</h3>
        <div className="friends-list-shell incoming">
          <div className="friends-list incoming-list">
            {incomingRequests.length === 0 ? <p className="friends-empty-copy">Новых заявок нет.</p> : null}
            {incomingRequests.map((request) => (
              <article key={String(request.requestId)} className="wpf-friend-card incoming">
                <div className="wpf-friend-avatar">
                  <AvatarImage url={request.avatarUrl} placeholder={request.avatarPlaceholder} alt={String(request.username)} />
                </div>
                <div className="wpf-friend-info">
                  <strong>{request.username}</strong>
                  <p>{request.subtitleText}</p>
                </div>
                <div className="wpf-friend-actions">
                  <button className="subtle-button compact" onClick={() => photinoBridge.sendCommand('friends.respond', { requestId: request.requestId, action: 'accept' })} type="button">Принять</button>
                  <button className="danger-button compact" onClick={() => photinoBridge.sendCommand('friends.respond', { requestId: request.requestId, action: 'decline' })} type="button">Отклонить</button>
                </div>
              </article>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
