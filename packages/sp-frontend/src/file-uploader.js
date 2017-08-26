import React, { Component } from "react";
import { watch, bindComponent } from "sp-components";

export class FileUploader extends Component {
  static propTypes = {
    SP: React.PropTypes.object
  };

  fileChanged(e) {
    const { SP } = this.props;
    const file = e.currentTarget.files[0];
    SP.files
      .create({
        name: file.name
      })
      .then(apiFile => {
        const uploadUrl = `${SP.schema.schemes[0]}://${SP.schema
          .host}/upload/${apiFile.id}?uploadKey=${apiFile.uploadKey}`;
        const data = new FormData();
        data.append("file", file);
        fetch(uploadUrl, {
          method: "POST",
          body: data
        })
          .then(() => {
            this.uploader.value = "";
          })
          .catch(err => {
            SP.error(err);
            this.uploader.value = "";
          });
      });
  }

  clickLink(e) {
    this.uploader.click();
  }

  render() {
    return (
      <div>
        <a onClick={e => this.clickLink(e)}>Upload MPEG-TS File</a>
        <input
          style={{ display: "none" }}
          ref={ref => (this.uploader = ref)}
          type="file"
          onChange={e => this.fileChanged(e)}
        />
      </div>
    );
  }
}

export default bindComponent(FileUploader);
