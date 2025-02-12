import React, { Fragment, useState } from 'react';
import CheckResultIcon from './CheckResultIcon';

const getResult = function (hosts, hostname) {
  return hostname in hosts ? hosts[hostname].result : 'unknown';
};

const RowGroup = ({ name, checks, clusterHosts }) => {
  const [open, setOpen] = useState(true);
  const emptyCells = Object.keys(clusterHosts)
    .map((key) => <td key={key} />)
    .concat(<td key="emptycell" />);

  return (
    <Fragment>
      <tr className="checks-table-row-group" onClick={() => setOpen(!open)}>
        <td className="checks-table-row-group-label">{name}</td>
        {emptyCells}
      </tr>
      {open &&
        checks.map(({ id, description, hosts }) => {
          return (
            <tr key={id}>
              <td>{description}</td>
              <td>{id}</td>
              {Object.keys(clusterHosts).map((hostname) => (
                <td key={hostname} className="align-center">
                  <CheckResultIcon result={getResult(hosts, hostname)} />
                </td>
              ))}
            </tr>
          );
        })}
    </Fragment>
  );
};

export default RowGroup;
